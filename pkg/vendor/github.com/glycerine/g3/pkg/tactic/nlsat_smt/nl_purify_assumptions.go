package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, update_eq_values and
// setup_assumptions.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// nlAssumptions owns each literal until Close; callers borrow literals when
// checking the cooperating SMT solver.
type nlAssumptions struct {
	manager  *ast.ASTManager
	refs     []ast.ASTRef
	literals []ast.ASTHandle
}

func (a *nlAssumptions) append(h ast.ASTHandle) error {
	ref, err := a.manager.NewRef(h)
	if err != nil {
		return err
	}
	a.refs = append(a.refs, ref)
	a.literals = append(a.literals, h)
	return nil
}

func evaluatedBoolean(m *ast.ASTManager, evaluator *model.ModelEvaluator, expression ast.ASTHandle) (value lbool.LBool, resultErr error) {
	ref, err := evaluator.EvaluateValue(expression)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&ref)) }()
	result, err := m.RefGet(ref)
	if err != nil {
		return lbool.Undef, err
	}
	if m.IsTrue(result) {
		return lbool.True, nil
	}
	if m.IsFalse(result) {
		return lbool.False, nil
	}
	return lbool.Undef, nil
}

func updateNLEqualityValues(m *ast.ASTManager, evaluator *model.ModelEvaluator, predicates []ast.ASTHandle) ([]lbool.LBool, error) {
	if m == nil || evaluator == nil {
		return nil, ast.ErrASTHandle
	}
	values := make([]lbool.LBool, len(predicates))
	for i, pred := range predicates {
		value, err := evaluatedBoolean(m, evaluator, pred)
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

func makeNLAssumptions(m *ast.ASTManager, evaluator *model.ModelEvaluator, context, predicates []ast.ASTHandle, polarities map[ast.ASTHandle]polarity, equalities []ast.ASTHandle, equalityValues []lbool.LBool) (_ *nlAssumptions, resultErr error) {
	if m == nil || evaluator == nil || len(equalities) != len(equalityValues) {
		return nil, ast.ErrASTDomain
	}
	a := &nlAssumptions{manager: m}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, a.Close())
		}
	}()
	for _, assumption := range context {
		if err := a.append(assumption); err != nil {
			return nil, err
		}
	}
	for _, pred := range predicates {
		pol, ok := polarities[pred]
		if !ok {
			return nil, ast.ErrASTDomain
		}
		value, err := evaluatedBoolean(m, evaluator, pred)
		if err != nil {
			return nil, err
		}
		if pol != negative && value == lbool.False {
			neg, err := m.MkNot(pred)
			if err != nil {
				return nil, err
			}
			if err = a.append(neg); err != nil {
				return nil, err
			}
		} else if pol != positive && value == lbool.True {
			if err = a.append(pred); err != nil {
				return nil, err
			}
		}
	}
	for i, pred := range equalities {
		switch equalityValues[i] {
		case lbool.True:
			if err := a.append(pred); err != nil {
				return nil, err
			}
		case lbool.False:
			neg, err := m.MkNot(pred)
			if err != nil {
				return nil, err
			}
			if err = a.append(neg); err != nil {
				return nil, err
			}
		}
	}
	return a, nil
}

func (a *nlAssumptions) Close() (result error) {
	if a == nil || a.manager == nil {
		return nil
	}
	for i := len(a.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, a.manager.DelRef(&a.refs[i]))
	}
	a.manager = nil
	a.refs = nil
	a.literals = nil
	return result
}
