package transforms

// Source: src/muz/transforms/dl_mk_interp_tail_simplifier.cpp/.h retained
// interpreted-tail rewriting and variable-equivalence propagation.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type InterpTailSimplifierPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	rewriter *astrewriter.TheoryRewriter
	priority uint32
	closed   bool
}

func NewInterpTailSimplifierPlugin(manager *ast.ASTManager, rules *base.RuleManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) (*InterpTailSimplifierPlugin, error) {
	if priority == 0 {
		priority = 40000
	}
	rewriter, err := astrewriter.NewTheoryRewriter(manager, context, globals, nil)
	if err != nil {
		return nil, err
	}
	return &InterpTailSimplifierPlugin{manager: manager, rules: rules, rewriter: rewriter, priority: priority}, nil
}

func (p *InterpTailSimplifierPlugin) Priority() uint32            { return p.priority }
func (p *InterpTailSimplifierPlugin) CanDestratifyNegation() bool { return false }
func (p *InterpTailSimplifierPlugin) Cancel()                     {}

func (p *InterpTailSimplifierPlugin) simplifyRule(rule *base.Rule) (_ *base.Rule, dropped, changed bool, resultErr error) {
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, false, false, err
	}
	total, err := rule.TailSize()
	if err != nil || uninterpreted == total {
		return nil, false, false, err
	}
	tails := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	for index := 0; index < uninterpreted; index++ {
		tail, negative, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, false, false, getErr
		}
		tails = append(tails, tail)
		negated = append(negated, negative)
	}
	interpreted := make([]ast.ASTHandle, 0, total-uninterpreted)
	for index := uninterpreted; index < total; index++ {
		tail, negative, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, false, false, getErr
		}
		if negative {
			return nil, false, false, fmt.Errorf("muz/transforms: interpreted rule tail retained negation metadata")
		}
		interpreted = append(interpreted, tail)
	}
	combined := interpreted[0]
	if len(interpreted) != 1 {
		combined, err = p.manager.MkAndArgs(interpreted)
		if err != nil {
			return nil, false, false, err
		}
		changed = true
	}
	out, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&out)) }()
	if err = p.rewriter.Rewrite(combined, out); err != nil {
		return nil, false, false, err
	}
	simplified, err := p.manager.RefGet(out)
	if err != nil {
		return nil, false, false, err
	}
	normalized, err := normalizeBooleanExpression(p.manager, simplified)
	if err != nil {
		return nil, false, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&normalized)) }()
	normalizedHandle, err := p.manager.RefGet(normalized)
	if err != nil {
		return nil, false, false, err
	}
	final, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&final)) }()
	if err = p.rewriter.Rewrite(normalizedHandle, final); err != nil {
		return nil, false, false, err
	}
	simplified, err = p.manager.RefGet(final)
	if err != nil {
		return nil, false, false, err
	}
	changed = changed || simplified != combined
	if p.manager.IsFalse(simplified) {
		return nil, true, true, nil
	}
	if !changed {
		return nil, false, false, nil
	}
	conjuncts, err := p.manager.FlattenAnd([]ast.ASTHandle{simplified})
	if err != nil {
		return nil, false, false, err
	}
	for _, conjunct := range conjuncts {
		if p.manager.IsTrue(conjunct) {
			continue
		}
		kind, getErr := p.manager.GetKind(conjunct)
		if getErr != nil {
			return nil, false, false, getErr
		}
		if kind == ast.ASTVar {
			truth, getErr := p.manager.MkTrue()
			if getErr != nil {
				return nil, false, false, getErr
			}
			conjunct, getErr = p.manager.MkEq(conjunct, truth)
			if getErr != nil {
				return nil, false, false, getErr
			}
		}
		tails = append(tails, conjunct)
		negated = append(negated, false)
	}
	head, err := rule.Head()
	if err != nil {
		return nil, false, false, err
	}
	name, err := rule.Name()
	if err != nil {
		return nil, false, false, err
	}
	replacement, err := p.rules.NewRule(head, tails, negated, name)
	if err != nil {
		return nil, false, false, err
	}
	return replacement, false, true, nil
}

func (p *InterpTailSimplifierPlugin) propagate(rule *base.Rule) (_ *base.Rule, changed bool, resultErr error) {
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, false, err
	}
	total, err := rule.TailSize()
	if err != nil || uninterpreted == total {
		return nil, false, err
	}
	variables, err := ruleVariableCount(p.manager, rule)
	if err != nil {
		return nil, false, err
	}
	subst, err := substitution.New(p.manager)
	if err != nil {
		return nil, false, err
	}
	unifier, err := substitution.NewUnifier(p.manager)
	if err != nil {
		return nil, false, errors.Join(err, subst.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, unifier.Close(), subst.Close()) }()
	if err = subst.Reserve(1, variables); err != nil {
		return nil, false, err
	}
	tryUnify := func(left, right ast.ASTHandle) (unified bool, resultErr error) {
		leftRef, makeErr := p.manager.NewRef(ast.ASTHandle{})
		if makeErr != nil {
			return false, makeErr
		}
		rightRef, makeErr := p.manager.NewRef(ast.ASTHandle{})
		if makeErr != nil {
			return false, errors.Join(makeErr, p.manager.DelRef(&leftRef))
		}
		defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&rightRef), p.manager.DelRef(&leftRef)) }()
		if makeErr = subst.ApplyExpression(left, leftRef); makeErr != nil {
			return false, makeErr
		}
		if makeErr = subst.ApplyExpression(right, rightRef); makeErr != nil {
			return false, makeErr
		}
		left, makeErr = p.manager.RefGet(leftRef)
		if makeErr != nil {
			return false, makeErr
		}
		right, makeErr = p.manager.RefGet(rightRef)
		if makeErr != nil {
			return false, makeErr
		}
		if makeErr = subst.ResetCache(); makeErr != nil {
			return false, makeErr
		}
		return unifier.UnifyPair(left, right, subst, false)
	}
	flexible := func(expression ast.ASTHandle) (bool, error) {
		kind, getErr := p.manager.GetKind(expression)
		if getErr != nil {
			return false, getErr
		}
		if kind == ast.ASTVar {
			return true, nil
		}
		return p.manager.IsValue(expression)
	}
	var todo []ast.ASTHandle
	for index := uninterpreted; index < total; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, false, getErr
		}
		todo = append(todo, tail)
	}
	for len(todo) != 0 {
		last := len(todo) - 1
		term := todo[last]
		todo = todo[:last]
		negative, bare, getErr := p.manager.IsNot(term)
		if getErr != nil {
			return nil, false, getErr
		}
		if negative {
			term = bare
		}
		kind, getErr := p.manager.GetKind(term)
		if getErr != nil {
			return nil, false, getErr
		}
		if kind == ast.ASTVar {
			value, valueErr := p.manager.MkBoolVal(!negative)
			if valueErr != nil {
				return nil, false, valueErr
			}
			unified, valueErr := tryUnify(term, value)
			if valueErr != nil {
				return nil, false, valueErr
			}
			changed = changed || unified
			continue
		}
		operation, getErr := p.manager.GetDeclKind(term)
		if getErr != nil {
			return nil, false, getErr
		}
		if !negative && operation == ast.ASTOpAnd {
			count, countErr := p.manager.GetNumArgs(term)
			if countErr != nil {
				return nil, false, countErr
			}
			for index := uint32(0); index < count; index++ {
				argument, argErr := p.manager.GetArg(term, index)
				if argErr != nil {
					return nil, false, argErr
				}
				todo = append(todo, argument)
			}
			continue
		}
		if operation != ast.ASTOpEq && operation != ast.ASTOpIff {
			continue
		}
		left, getErr := p.manager.GetArg(term, 0)
		if getErr != nil {
			return nil, false, getErr
		}
		right, getErr := p.manager.GetArg(term, 1)
		if getErr != nil {
			return nil, false, getErr
		}
		if operation == ast.ASTOpIff {
			for {
				negated, bare, e := p.manager.IsNot(left)
				if e != nil {
					return nil, false, e
				}
				if !negated {
					break
				}
				left = bare
				negative = !negative
			}
			for {
				negated, bare, e := p.manager.IsNot(right)
				if e != nil {
					return nil, false, e
				}
				if !negated {
					break
				}
				right = bare
				negative = !negative
			}
			leftKind, e := p.manager.GetKind(left)
			if e != nil {
				return nil, false, e
			}
			if leftKind != ast.ASTVar {
				left, right = right, left
			}
		}
		leftFlex, getErr := flexible(left)
		if getErr != nil {
			return nil, false, getErr
		}
		rightFlex, getErr := flexible(right)
		if getErr != nil {
			return nil, false, getErr
		}
		if !leftFlex || !rightFlex {
			continue
		}
		leftKind, getErr := p.manager.GetKind(left)
		if getErr != nil {
			return nil, false, getErr
		}
		var target ast.ASTHandle
		switch {
		case operation == ast.ASTOpEq && !negative:
			target = right
		case operation == ast.ASTOpIff && leftKind == ast.ASTVar && !negative:
			target = right
		case operation == ast.ASTOpIff && leftKind == ast.ASTVar && negative && p.manager.IsTrue(right):
			target, getErr = p.manager.MkFalse()
		case operation == ast.ASTOpIff && leftKind == ast.ASTVar && negative && p.manager.IsFalse(right):
			target, getErr = p.manager.MkTrue()
		default:
			continue
		}
		if getErr != nil {
			return nil, false, getErr
		}
		unified, unifyErr := tryUnify(left, target)
		if unifyErr != nil {
			return nil, false, unifyErr
		}
		changed = changed || unified
	}
	if !changed {
		return nil, false, nil
	}
	var references []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(p.manager, references)) }()
	apply := func(expression ast.ASTHandle) (ast.ASTHandle, error) {
		ref, applyErr := p.manager.NewRef(ast.ASTHandle{})
		if applyErr != nil {
			return ast.ASTHandle{}, applyErr
		}
		references = append(references, ref)
		if applyErr = subst.ApplyExpression(expression, ref); applyErr != nil {
			return ast.ASTHandle{}, applyErr
		}
		return p.manager.RefGet(ref)
	}
	head, err := rule.Head()
	if err != nil {
		return nil, false, err
	}
	head, err = apply(head)
	if err != nil {
		return nil, false, err
	}
	tails := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	for index := 0; index < total; index++ {
		tail, negative, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, false, getErr
		}
		tail, getErr = apply(tail)
		if getErr != nil {
			return nil, false, getErr
		}
		duplicate := false
		for prior := range tails {
			if tails[prior] == tail && negated[prior] == negative {
				duplicate = true
				break
			}
		}
		if !duplicate {
			tails = append(tails, tail)
			negated = append(negated, negative)
		}
	}
	name, err := rule.Name()
	if err != nil {
		return nil, false, err
	}
	replacement, err := p.rules.NewRule(head, tails, negated, name)
	if err != nil {
		return nil, false, err
	}
	return replacement, true, nil
}

func (p *InterpTailSimplifierPlugin) transformRule(source *base.Rule) (_ *base.Rule, dropped, changed bool, resultErr error) {
	head, err := source.Head()
	if err != nil {
		return nil, false, false, err
	}
	hasQuantifiers, err := p.manager.HasQuantifiers(head)
	if err != nil {
		return nil, false, false, err
	}
	total, err := source.TailSize()
	for index := 0; err == nil && !hasQuantifiers && index < total; index++ {
		var tail ast.ASTHandle
		tail, _, err = source.Tail(index)
		if err == nil {
			hasQuantifiers, err = p.manager.HasQuantifiers(tail)
		}
	}
	if err != nil || hasQuantifiers {
		return nil, false, false, err
	}
	current := source
	owned := false
	defer func() {
		if owned && resultErr != nil {
			resultErr = errors.Join(resultErr, current.Close())
		}
	}()
	for {
		simplified, drop, simplifiedChanged, err := p.simplifyRule(current)
		if err != nil {
			return nil, false, changed, err
		}
		if drop {
			if owned {
				resultErr = current.Close()
				owned = false
			}
			return nil, true, true, resultErr
		}
		if simplifiedChanged {
			if owned {
				if err = current.Close(); err != nil {
					return nil, false, changed, err
				}
			}
			current, owned, changed = simplified, true, true
		}
		propagated, propagatedChanged, err := p.propagate(current)
		if err != nil {
			return nil, false, changed, err
		}
		if !propagatedChanged {
			if !owned {
				return nil, false, changed, nil
			}
			owned = false
			return current, false, changed, nil
		}
		if owned {
			if err = current.Close(); err != nil {
				return nil, false, changed, errors.Join(err, propagated.Close())
			}
		}
		current, owned, changed = propagated, true, true
	}
}

func (p *InterpTailSimplifierPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil || p.rewriter == nil {
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
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	changed := false
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		replacement, dropped, ruleChanged, transformErr := p.transformRule(rule)
		changed = changed || ruleChanged
		if transformErr == nil && !dropped {
			if replacement == nil {
				replacement = rule
			}
			transformErr = result.Add(replacement)
		}
		transformErr = errors.Join(transformErr, rule.Close())
		if replacement != nil && replacement != rule {
			transformErr = errors.Join(transformErr, replacement.Close())
		}
		if transformErr != nil {
			return nil, transformErr
		}
	}
	if !changed {
		return nil, nil
	}
	keep = true
	return result, nil
}

func (p *InterpTailSimplifierPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	if p.closed {
		return nil
	}
	p.closed = true
	if p.rewriter != nil {
		err := p.rewriter.Close()
		p.rewriter = nil
		return err
	}
	return nil
}
