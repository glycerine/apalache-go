package transforms

// Source: src/muz/transforms/dl_mk_coi_filter.cpp and .h. This file ports the
// retained rule-pruning core; fixedpoint-context model conversion remains with
// the future context integration.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/dataflow"
)

func newConeResult(source *base.RuleSet) (*base.RuleSet, error) {
	manager, err := source.RuleManager()
	if err != nil {
		return nil, err
	}
	result, err := base.NewRuleSet(manager)
	if err != nil {
		return nil, err
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

func finishConeResult(result *base.RuleSet) error {
	valid, err := result.Finalize()
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("muz/transforms: cone filter produced an unstratified rule set")
	}
	return nil
}

func topDownCone(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	reachable, err := dataflow.TopDown(source)
	if err != nil {
		return nil, err
	}
	result, err := newConeResult(source)
	if err != nil {
		return nil, err
	}
	keepResult := false
	defer func() {
		if !keepResult || resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	kept := 0
	for i := 0; i < count; i++ {
		rule, e := source.Rule(i)
		if e != nil {
			return nil, e
		}
		head, e := rule.HeadDeclaration()
		if e == nil && reachable.IsReachable(head) {
			e = result.Add(rule)
			if e == nil {
				kept++
			}
		}
		if cleanup := rule.Close(); cleanup != nil {
			e = errors.Join(e, cleanup)
		}
		if e != nil {
			return nil, e
		}
	}
	if kept == count {
		return nil, nil
	}
	if err = finishConeResult(result); err != nil {
		return nil, err
	}
	keepResult = true
	return result, nil
}

func filteredRule(manager *base.RuleManager, rule *base.Rule, reachable *dataflow.Reachability) (*base.Rule, bool, error) {
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, false, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, false, err
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return nil, false, err
	}
	removed := false
	for i := 0; i < positive; i++ {
		declaration, e := rule.TailDeclaration(i)
		if e != nil {
			return nil, false, e
		}
		if !reachable.IsReachable(declaration) {
			return nil, false, nil
		}
	}
	tail := make([]ast.ASTHandle, 0, tailSize)
	negated := make([]bool, 0, tailSize)
	for i := 0; i < tailSize; i++ {
		expression, negative, e := rule.Tail(i)
		if e != nil {
			return nil, false, e
		}
		if i < uninterpreted && negative {
			declaration, x := rule.TailDeclaration(i)
			if x != nil {
				return nil, false, x
			}
			if !reachable.IsReachable(declaration) {
				removed = true
				continue
			}
		}
		tail = append(tail, expression)
		negated = append(negated, negative)
	}
	if !removed {
		return nil, true, nil
	}
	head, err := rule.Head()
	if err != nil {
		return nil, false, err
	}
	name, err := rule.Name()
	if err != nil {
		return nil, false, err
	}
	replacement, err := manager.NewRule(head, tail, negated, name)
	return replacement, true, err
}

func bottomUpCone(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	reachable, err := dataflow.BottomUp(source)
	if err != nil {
		return nil, err
	}
	manager, err := source.RuleManager()
	if err != nil {
		return nil, err
	}
	result, err := newConeResult(source)
	if err != nil {
		return nil, err
	}
	keepResult := false
	defer func() {
		if !keepResult || resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	kept := 0
	for i := 0; i < count; i++ {
		rule, e := source.Rule(i)
		if e != nil {
			return nil, e
		}
		replacement, retain, e := filteredRule(manager, rule, reachable)
		if e == nil && retain {
			candidate := rule
			if replacement != nil {
				candidate = replacement
			}
			e = result.Add(candidate)
			if e == nil {
				kept++
			}
		}
		if replacement != nil {
			e = errors.Join(e, replacement.Close())
		}
		e = errors.Join(e, rule.Close())
		if e != nil {
			return nil, e
		}
	}
	// Preserve the source transform's observable no-change test, which compares
	// rule counts even when an unreachable negative tail was simplified.
	if kept == count {
		return nil, nil
	}
	if err = finishConeResult(result); err != nil {
		return nil, err
	}
	keepResult = true
	return result, nil
}

// ConeOfInfluence applies the source top-down then bottom-up passes. A nil
// result means neither pass changed the source. A non-nil result is owned by
// the caller and is already finalized.
func ConeOfInfluence(source *base.RuleSet) (*base.RuleSet, error) {
	top, err := topDownCone(source)
	if err != nil {
		return nil, err
	}
	basis := source
	if top != nil {
		basis = top
	}
	bottom, err := bottomUpCone(basis)
	if err != nil {
		if top != nil {
			err = errors.Join(err, top.Close())
		}
		return nil, err
	}
	if bottom != nil {
		if top != nil {
			err = top.Close()
			if err != nil {
				return nil, errors.Join(err, bottom.Close())
			}
		}
		return bottom, nil
	}
	return top, nil
}
