package transforms

// Source: src/muz/transforms/dl_mk_coalesce.cpp/.h shared-body rule merging.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

type RuleCoalescePlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	priority uint32
	closed   bool
}

func NewRuleCoalescePlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *RuleCoalescePlugin {
	if priority == 0 {
		priority = 50
	}
	return &RuleCoalescePlugin{manager: manager, rules: rules, priority: priority}
}

func (p *RuleCoalescePlugin) Priority() uint32            { return p.priority }
func (p *RuleCoalescePlugin) CanDestratifyNegation() bool { return false }
func (p *RuleCoalescePlugin) Cancel()                     {}

func (p *RuleCoalescePlugin) sameBody(left, right *base.Rule) (bool, error) {
	leftHead, err := left.HeadDeclaration()
	if err != nil {
		return false, err
	}
	rightHead, err := right.HeadDeclaration()
	if err != nil || leftHead != rightHead {
		return false, err
	}
	leftSize, err := left.UninterpretedTailSize()
	if err != nil {
		return false, err
	}
	rightSize, err := right.UninterpretedTailSize()
	if err != nil || leftSize != rightSize {
		return false, err
	}
	for index := 0; index < leftSize; index++ {
		leftDecl, getErr := left.TailDeclaration(index)
		if getErr != nil {
			return false, getErr
		}
		rightDecl, getErr := right.TailDeclaration(index)
		if getErr != nil || leftDecl != rightDecl {
			return false, getErr
		}
		_, leftNegated, getErr := left.Tail(index)
		if getErr != nil {
			return false, getErr
		}
		_, rightNegated, getErr := right.Tail(index)
		if getErr != nil || leftNegated != rightNegated {
			return false, getErr
		}
	}
	return true, nil
}

func (p *RuleCoalescePlugin) generalizeApplications(left, right ast.ASTHandle, next *uint32, leftSub, rightSub *[]ast.ASTHandle) (ast.ASTHandle, error) {
	leftDecl, err := p.manager.GetDecl(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightDecl, err := p.manager.GetDecl(right)
	if err != nil || leftDecl != rightDecl {
		return ast.ASTHandle{}, errors.Join(err, ast.ErrASTDomain)
	}
	arity, err := p.manager.GetNumArgs(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightArity, err := p.manager.GetNumArgs(right)
	if err != nil || arity != rightArity {
		return ast.ASTHandle{}, errors.Join(err, ast.ErrASTDomain)
	}
	arguments := make([]ast.ASTHandle, arity)
	for index := uint32(0); index < arity; index++ {
		leftArg, getErr := p.manager.GetArg(left, index)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		rightArg, getErr := p.manager.GetArg(right, index)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		leftSort, getErr := p.manager.GetSort(leftArg)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		rightSort, getErr := p.manager.GetSort(rightArg)
		if getErr != nil || leftSort != rightSort {
			return ast.ASTHandle{}, errors.Join(getErr, ast.ErrASTDomain)
		}
		*leftSub = append(*leftSub, leftArg)
		*rightSub = append(*rightSub, rightArg)
		arguments[index], getErr = p.manager.MkVar(*next, leftSort)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		*next++
	}
	return p.manager.MkApp(leftDecl, arguments)
}

func (p *RuleCoalescePlugin) ruleVariableSorts(rule *base.Rule) ([]ast.ASTHandle, error) {
	used, err := ast.NewASTUsedVars(p.manager)
	if err != nil {
		return nil, err
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	used.Reset()
	if err = used.Process(head); err != nil {
		return nil, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	for index := 0; index < total; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, getErr
		}
		if getErr = used.Process(tail); getErr != nil {
			return nil, getErr
		}
	}
	sorts := make([]ast.ASTHandle, used.MaxFoundVarIndexPlusOne())
	for index := range sorts {
		sorts[index] = used.Contains(uint32(index))
	}
	return sorts, nil
}

func (p *RuleCoalescePlugin) extractConjunction(sub []ast.ASTHandle, rule *base.Rule, next *uint32) (_ ast.ASTRef, resultErr error) {
	sorts, err := p.ruleVariableSorts(rule)
	if err != nil {
		return ast.ASTRef{}, err
	}
	reverse := make([]ast.ASTHandle, len(sorts))
	seen := make([]bool, len(sorts))
	conjuncts := make([]ast.ASTHandle, 0, len(sub))
	for index, expression := range sub {
		sort, getErr := p.manager.GetSort(expression)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		general, getErr := p.manager.MkVar(uint32(index), sort)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		kind, getErr := p.manager.GetKind(expression)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		if kind == ast.ASTVar {
			variable, getErr := p.manager.GetVarIndex(expression)
			if getErr != nil || int(variable) >= len(reverse) {
				return ast.ASTRef{}, errors.Join(getErr, ast.ErrASTDomain)
			}
			if !seen[variable] {
				reverse[variable] = general
				seen[variable] = true
			} else {
				equality, equalErr := p.manager.MkEq(reverse[variable], general)
				if equalErr != nil {
					return ast.ASTRef{}, equalErr
				}
				conjuncts = append(conjuncts, equality)
			}
		} else {
			value, valueErr := p.manager.IsValue(expression)
			if valueErr != nil || !value {
				return ast.ASTRef{}, errors.Join(valueErr, ast.ErrASTDomain)
			}
			equality, equalErr := p.manager.MkEq(expression, general)
			if equalErr != nil {
				return ast.ASTRef{}, equalErr
			}
			conjuncts = append(conjuncts, equality)
		}
	}
	for index, sort := range sorts {
		if sort.IsNull() || seen[index] {
			continue
		}
		reverse[index], err = p.manager.MkVar(*next, sort)
		if err != nil {
			return ast.ASTRef{}, err
		}
		*next++
	}
	substitute, err := astrewriter.NewVarSubstitution(p.manager, false)
	if substitute != nil {
		defer func() { resultErr = errors.Join(resultErr, substitute.Close()) }()
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	total, totalErr := rule.TailSize()
	if err != nil || totalErr != nil {
		return ast.ASTRef{}, errors.Join(err, totalErr)
	}
	var temporary []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(p.manager, temporary)) }()
	for index := uninterpreted; index < total; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		updated, getErr := p.manager.NewRef(ast.ASTHandle{})
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		temporary = append(temporary, updated)
		if getErr = substitute.Substitute(tail, reverse, updated); getErr != nil {
			return ast.ASTRef{}, getErr
		}
		updatedHandle, getErr := p.manager.RefGet(updated)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		conjuncts = append(conjuncts, updatedHandle)
	}
	combined, err := p.manager.MkAndArgs(conjuncts)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.manager.NewRef(combined)
}

func (p *RuleCoalescePlugin) merge(target, source *base.Rule) (_ *base.Rule, resultErr error) {
	var leftSub, rightSub []ast.ASTHandle
	var next uint32
	sourceHead, err := source.Head()
	if err != nil {
		return nil, err
	}
	targetHead, err := target.Head()
	if err != nil {
		return nil, err
	}
	head, err := p.generalizeApplications(sourceHead, targetHead, &next, &leftSub, &rightSub)
	if err != nil {
		return nil, err
	}
	uninterpreted, err := source.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	tails := make([]ast.ASTHandle, 0, uninterpreted+1)
	negated := make([]bool, 0, uninterpreted+1)
	for index := 0; index < uninterpreted; index++ {
		left, leftNegated, getErr := source.Tail(index)
		if getErr != nil {
			return nil, getErr
		}
		right, _, getErr := target.Tail(index)
		if getErr != nil {
			return nil, getErr
		}
		general, getErr := p.generalizeApplications(left, right, &next, &leftSub, &rightSub)
		if getErr != nil {
			return nil, getErr
		}
		tails = append(tails, general)
		negated = append(negated, leftNegated)
	}
	leftFormula, err := p.extractConjunction(leftSub, source, &next)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&leftFormula)) }()
	rightFormula, err := p.extractConjunction(rightSub, target, &next)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&rightFormula)) }()
	leftHandle, err := p.manager.RefGet(leftFormula)
	if err != nil {
		return nil, err
	}
	rightHandle, err := p.manager.RefGet(rightFormula)
	if err != nil {
		return nil, err
	}
	disjunction, err := p.manager.MkOrArgs([]ast.ASTHandle{leftHandle, rightHandle})
	if err != nil {
		return nil, err
	}
	tails = append(tails, disjunction)
	negated = append(negated, false)
	name, err := target.Name()
	if err != nil {
		return nil, err
	}
	merged, err := p.rules.NewRule(head, tails, negated, name)
	if err != nil {
		return nil, fmt.Errorf("muz/transforms: construct coalesced rule: %w", err)
	}
	return merged, nil
}

func (p *RuleCoalescePlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	buckets := make(map[ast.ASTHandle][]*base.Rule)
	var order []ast.ASTHandle
	defer func() { resultErr = errors.Join(resultErr, closeRuleBuckets(buckets)) }()
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		head, getErr := rule.HeadDeclaration()
		if getErr != nil {
			return nil, errors.Join(getErr, rule.Close())
		}
		if len(buckets[head]) == 0 {
			order = append(order, head)
		}
		buckets[head] = append(buckets[head], rule)
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
	for _, head := range order {
		rules := append([]*base.Rule(nil), buckets[head]...)
		for left := 0; left < len(rules); left++ {
			target, retainErr := rules[left].Retain()
			if retainErr != nil {
				return nil, retainErr
			}
			for right := left + 1; right < len(rules); {
				same, sameErr := p.sameBody(target, rules[right])
				if sameErr != nil {
					return nil, errors.Join(sameErr, target.Close())
				}
				if !same {
					right++
					continue
				}
				merged, mergeErr := p.merge(target, rules[right])
				if mergeErr != nil {
					return nil, errors.Join(mergeErr, target.Close())
				}
				if mergeErr = target.Close(); mergeErr != nil {
					return nil, errors.Join(mergeErr, merged.Close())
				}
				target = merged
				rules[right] = rules[len(rules)-1]
				rules = rules[:len(rules)-1]
			}
			if err = errors.Join(result.Add(target), target.Close()); err != nil {
				return nil, err
			}
		}
	}
	keep = true
	return result, nil
}

func (p *RuleCoalescePlugin) Close() error {
	if p == nil || p.closed {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	p.manager = nil
	p.rules = nil
	return nil
}
