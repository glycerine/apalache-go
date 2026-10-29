package transforms

// Source: src/muz/transforms/dl_mk_backwards.cpp/.h backward-flow Horn rule
// construction used by Karr invariant discovery.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type BackwardsPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	priority uint32
	closed   bool
}

func NewBackwardsPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *BackwardsPlugin {
	return &BackwardsPlugin{manager: manager, rules: rules, priority: priority}
}

func (p *BackwardsPlugin) Priority() uint32            { return p.priority }
func (p *BackwardsPlugin) CanDestratifyNegation() bool { return false }
func (p *BackwardsPlugin) Cancel()                     {}

func (p *BackwardsPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	query, err := p.manager.MkFreshConst("Q", boolean)
	if err != nil {
		return nil, err
	}
	queryRef, err := p.manager.NewRef(query)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&queryRef)) }()
	queryDeclaration, err := p.manager.GetDecl(query)
	if err != nil {
		return nil, err
	}
	if err = p.rules.RegisterPredicate(queryDeclaration); err != nil {
		return nil, err
	}
	if err = result.SetOutputPredicate(queryDeclaration); err != nil {
		return nil, err
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		uninterpreted, sizeErr := rule.UninterpretedTailSize()
		total, totalErr := rule.TailSize()
		getErr = errors.Join(sizeErr, totalErr)
		var tails []ast.ASTHandle
		if getErr == nil {
			declaration, declarationErr := rule.HeadDeclaration()
			var output bool
			if declarationErr == nil {
				output, declarationErr = source.IsOutputPredicate(declaration)
			}
			if declarationErr == nil && !output {
				var head ast.ASTHandle
				head, declarationErr = rule.Head()
				tails = append(tails, head)
			}
			getErr = declarationErr
		}
		for j := uninterpreted; getErr == nil && j < total; j++ {
			tail, _, tailErr := rule.Tail(j)
			getErr = tailErr
			tails = append(tails, tail)
		}
		name, nameErr := rule.Name()
		getErr = errors.Join(getErr, nameErr)
		for j := 0; getErr == nil && j <= uninterpreted; j++ {
			if j == uninterpreted && j > 0 {
				break
			}
			head := query
			if j < uninterpreted {
				head, _, getErr = rule.Tail(j)
			}
			var replacement *base.Rule
			if getErr == nil {
				replacement, getErr = p.rules.NewRule(head, tails, make([]bool, len(tails)), name)
			}
			if getErr == nil {
				getErr = result.Add(replacement)
			}
			if replacement != nil {
				getErr = errors.Join(getErr, replacement.Close())
			}
		}
		getErr = errors.Join(getErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
	}
	keep = true
	return result, nil
}

func (p *BackwardsPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true
	return nil
}
