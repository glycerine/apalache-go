package transforms

// Source: src/muz/transforms/dl_mk_unfold.cpp/.h one-round rule unfolding.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// RuleUnfoldPlugin expands every original uninterpreted tail once. Tails
// introduced by a definition are skipped in the current round, matching the
// source's tail-index advance by the definition's uninterpreted-tail size.
type RuleUnfoldPlugin struct {
	resolver *RuleInlinerPlugin
	priority uint32
	closed   bool
}

func NewRuleUnfoldPlugin(manager *ast.ASTManager, rules *base.RuleManager, params *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) *RuleUnfoldPlugin {
	if priority == 0 {
		priority = 100
	}
	return &RuleUnfoldPlugin{
		resolver: NewRuleInlinerPluginWithServices(manager, rules, nil, false, false, params, globals, priority),
		priority: priority,
	}
}

func (p *RuleUnfoldPlugin) Priority() uint32            { return p.priority }
func (p *RuleUnfoldPlugin) CanDestratifyNegation() bool { return false }
func (p *RuleUnfoldPlugin) Cancel()                     {}

func (p *RuleUnfoldPlugin) expand(rule *base.Rule, tailIndex int, source, destination *base.RuleSet) (resultErr error) {
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return err
	}
	if tailIndex >= uninterpreted {
		return destination.Add(rule)
	}
	declaration, err := rule.TailDeclaration(tailIndex)
	if err != nil {
		return err
	}
	definitions, err := source.PredicateRules(declaration)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, closeRules(definitions)) }()
	for _, definition := range definitions {
		definitionTails, getErr := definition.UninterpretedTailSize()
		if getErr != nil {
			return getErr
		}
		resolved, resolveErr := p.resolver.resolve(rule, tailIndex, definition)
		if resolveErr != nil {
			return resolveErr
		}
		if resolved == nil {
			continue
		}
		resolveErr = p.expand(resolved, tailIndex+definitionTails, source, destination)
		resolveErr = errors.Join(resolveErr, resolved.Close())
		if resolveErr != nil {
			return resolveErr
		}
	}
	return nil
}

func (p *RuleUnfoldPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.resolver == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	result, err := base.NewRuleSet(p.resolver.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		getErr = errors.Join(p.expand(rule, 0, source, result), rule.Close())
		if getErr != nil {
			return nil, getErr
		}
	}
	keep = true
	return result, nil
}

func (p *RuleUnfoldPlugin) Close() error {
	if p == nil || p.closed {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	err := p.resolver.Close()
	p.resolver = nil
	return err
}
