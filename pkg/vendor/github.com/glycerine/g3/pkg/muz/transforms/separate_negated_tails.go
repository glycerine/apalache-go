package transforms

// Source: src/muz/transforms/dl_mk_separate_negated_tails.cpp/.h.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// SeparateNegatedTailsPlugin removes variables private to a negated predicate
// by introducing a fresh positive predicate for its existential negation.
type SeparateNegatedTailsPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	priority uint32
	fresh    []ast.ASTHandle
	closed   bool
}

func NewSeparateNegatedTailsPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *SeparateNegatedTailsPlugin {
	if priority == 0 {
		priority = 21000
	}
	return &SeparateNegatedTailsPlugin{manager: manager, rules: rules, priority: priority}
}

// FreshPredicates returns borrowed abstraction declarations in creation order.
func (p *SeparateNegatedTailsPlugin) FreshPredicates() []ast.ASTHandle {
	if p == nil || p.closed {
		return nil
	}
	return append([]ast.ASTHandle(nil), p.fresh...)
}

func (p *SeparateNegatedTailsPlugin) Priority() uint32            { return p.priority }
func (p *SeparateNegatedTailsPlugin) CanDestratifyNegation() bool { return false }
func (p *SeparateNegatedTailsPlugin) Cancel()                     {}

func (p *SeparateNegatedTailsPlugin) privateVariables(rule *base.Rule, selected int) (map[ast.ASTHandle]bool, error) {
	used, err := ast.NewASTUsedVars(p.manager)
	if err != nil {
		return nil, err
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	if err = used.Process(head); err != nil {
		return nil, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	for index := 0; index < total; index++ {
		if index == selected {
			continue
		}
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, getErr
		}
		if err = used.Process(tail); err != nil {
			return nil, err
		}
	}
	tail, _, err := rule.Tail(selected)
	if err != nil {
		return nil, err
	}
	count, err := p.manager.GetNumArgs(tail)
	if err != nil {
		return nil, err
	}
	private := make(map[ast.ASTHandle]bool)
	for index := uint32(0); index < count; index++ {
		argument, getErr := p.manager.GetArg(tail, index)
		if getErr != nil {
			return nil, getErr
		}
		kind, getErr := p.manager.GetKind(argument)
		if getErr != nil {
			return nil, getErr
		}
		if kind != ast.ASTVar {
			continue
		}
		variable, getErr := p.manager.GetVarIndex(argument)
		if getErr != nil {
			return nil, getErr
		}
		if used.Contains(variable).IsNull() {
			private[argument] = true
		}
	}
	return private, nil
}

func (p *SeparateNegatedTailsPlugin) abstractPredicate(tail ast.ASTHandle, private map[ast.ASTHandle]bool, result *base.RuleSet) (_ ast.ASTHandle, pin ast.ASTRef, resultErr error) {
	count, err := p.manager.GetNumArgs(tail)
	if err != nil {
		return ast.ASTHandle{}, pin, err
	}
	arguments := make([]ast.ASTHandle, 0, count)
	domain := make([]ast.ASTHandle, 0, count)
	for index := uint32(0); index < count; index++ {
		argument, getErr := p.manager.GetArg(tail, index)
		if getErr != nil {
			return ast.ASTHandle{}, pin, getErr
		}
		if private[argument] {
			continue
		}
		sort, getErr := p.manager.GetSort(argument)
		if getErr != nil {
			return ast.ASTHandle{}, pin, getErr
		}
		arguments = append(arguments, argument)
		domain = append(domain, sort)
	}
	declaration, err := p.manager.GetDecl(tail)
	if err != nil {
		return ast.ASTHandle{}, pin, err
	}
	prefix, err := p.manager.GetASTName(declaration)
	if err != nil {
		return ast.ASTHandle{}, pin, err
	}
	suffix, err := p.manager.Symbols.Intern("N")
	if err != nil {
		return ast.ASTHandle{}, pin, err
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, pin, err
	}
	fresh, err := p.manager.MkFreshFuncDecl(prefix, suffix, domain, boolean)
	if err != nil {
		return ast.ASTHandle{}, pin, err
	}
	if err = p.rules.RegisterPredicate(fresh); err != nil {
		return ast.ASTHandle{}, pin, err
	}
	p.fresh = append(p.fresh, fresh)
	abstraction, err := p.manager.MkApp(fresh, arguments)
	if err != nil {
		return ast.ASTHandle{}, pin, err
	}
	pin, err = p.manager.NewRef(abstraction)
	if err != nil {
		return ast.ASTHandle{}, pin, err
	}
	keepPin := false
	defer func() {
		if !keepPin {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&pin))
		}
	}()
	definition, err := p.rules.NewRule(abstraction, []ast.ASTHandle{tail}, []bool{true}, "")
	if err != nil {
		return ast.ASTHandle{}, pin, err
	}
	defer func() { resultErr = errors.Join(resultErr, definition.Close()) }()
	if err = result.Add(definition); err != nil {
		return ast.ASTHandle{}, pin, err
	}
	keepPin = true
	return abstraction, pin, nil
}

func (p *SeparateNegatedTailsPlugin) rewriteRule(rule *base.Rule, result *base.RuleSet) (resultErr error) {
	var pins []ast.ASTRef
	defer func() {
		for index := len(pins) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&pins[index]))
		}
	}()
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return err
	}
	total, err := rule.TailSize()
	if err != nil {
		return err
	}
	tails := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	for index := 0; index < positive; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return getErr
		}
		tails = append(tails, tail)
		negated = append(negated, false)
	}
	for index := positive; index < uninterpreted; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return getErr
		}
		private, getErr := p.privateVariables(rule, index)
		if getErr != nil {
			return fmt.Errorf("private variables for tail %d: %w", index, getErr)
		}
		if len(private) != 0 {
			var pin ast.ASTRef
			tail, pin, getErr = p.abstractPredicate(tail, private, result)
			if getErr != nil {
				return fmt.Errorf("abstract tail %d: %w", index, getErr)
			}
			pins = append(pins, pin)
			negated = append(negated, false)
		} else {
			negated = append(negated, true)
		}
		tails = append(tails, tail)
	}
	for index := uninterpreted; index < total; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return getErr
		}
		tails = append(tails, tail)
		negated = append(negated, false)
	}
	head, err := rule.Head()
	if err != nil {
		return err
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return err
	}
	rewritten, err := p.rules.NewRuleWithSymbol(head, tails, negated, name)
	if err != nil {
		return fmt.Errorf("construct rewritten rule: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, rewritten.Close()) }()
	return result.Add(rewritten)
}

func (p *SeparateNegatedTailsPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	p.fresh = nil
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
	changed := false
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		positive, sizeErr := rule.PositiveTailSize()
		uninterpreted, uninterpretedErr := rule.UninterpretedTailSize()
		getErr = errors.Join(sizeErr, uninterpretedErr)
		rewrite := false
		for tailIndex := positive; getErr == nil && tailIndex < uninterpreted; tailIndex++ {
			private, privateErr := p.privateVariables(rule, tailIndex)
			getErr = privateErr
			if len(private) != 0 {
				rewrite = true
				break
			}
		}
		if getErr == nil && rewrite {
			getErr = p.rewriteRule(rule, result)
			if getErr != nil {
				getErr = fmt.Errorf("muz/transforms: separate negated tails rewrite: %w", getErr)
			}
			changed = true
		} else if getErr == nil {
			getErr = result.Add(rule)
		}
		getErr = errors.Join(getErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
	}
	if !changed {
		return nil, nil
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, fmt.Errorf("muz/transforms: separate negated tails inherit predicates: %w", err)
	}
	keep = true
	return result, nil
}

func (p *SeparateNegatedTailsPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true
	p.fresh = nil
	return nil
}
