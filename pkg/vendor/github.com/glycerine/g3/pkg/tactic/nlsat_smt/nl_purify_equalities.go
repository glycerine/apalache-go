package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, enforce_equalities.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type nlEqualityPair struct{ left, right ast.ASTHandle }

type nlEqualityState struct {
	manager    *ast.ASTManager
	pairs      map[nlEqualityPair]ast.ASTHandle
	predicates []ast.ASTHandle
	values     []lbool.LBool
	refs       []ast.ASTRef
	filter     *tactic.FilterModelConverter
}

func newNLEqualityState(m *ast.ASTManager, filter *tactic.FilterModelConverter) (*nlEqualityState, error) {
	if m == nil || filter == nil {
		return nil, ast.ErrASTHandle
	}
	return &nlEqualityState{manager: m, filter: filter, pairs: make(map[nlEqualityPair]ast.ASTHandle)}, nil
}

func (s *nlEqualityState) hold(h ast.ASTHandle) error {
	ref, err := s.manager.NewRef(h)
	if err == nil {
		s.refs = append(s.refs, ref)
	}
	return err
}

// enforce compares interface values in the model and links each newly found
// equal pair in both solvers. The caller owns interfaces and solver objects.
func (s *nlEqualityState) enforce(evaluator *model.ModelEvaluator, interfaces []ast.ASTHandle, nonlinear *tactic.Goal, smt *smtcore.SMTSolver) (changed bool, resultErr error) {
	if s == nil || s.manager == nil || evaluator == nil || nonlinear == nil || nonlinear.Manager() != s.manager || smt == nil {
		return false, ast.ErrASTDomain
	}
	if err := nonlinear.CheckOwner(); err != nil {
		return false, err
	}
	m := s.manager
	var values []ast.ASTRef
	defer func() {
		for i := len(values) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&values[i]))
		}
	}()
	byValue := make(map[ast.ASTHandle]ast.ASTHandle)
	for _, w := range interfaces {
		ref, err := evaluator.EvaluateValue(w)
		if err != nil {
			return false, err
		}
		values = append(values, ref)
		value, err := m.RefGet(ref)
		if err != nil {
			return false, err
		}
		v, found := byValue[value]
		if !found {
			byValue[value] = w
			continue
		}
		pair := nlEqualityPair{v, w}
		if _, found = s.pairs[pair]; found {
			continue
		}
		boolean, err := m.MkBoolSort()
		if err != nil {
			return false, err
		}
		pred, err := m.MkFreshConstNullable(nil, boolean)
		if err != nil {
			return false, err
		}
		if err = s.hold(pred); err != nil {
			return false, err
		}
		decl, err := m.GetDecl(pred)
		if err != nil {
			return false, err
		}
		if err = s.filter.Insert(decl); err != nil {
			return false, err
		}
		equality, err := m.MkEq(w, v)
		if err != nil {
			return false, err
		}
		if err = s.hold(equality); err != nil {
			return false, err
		}
		notPred, err := m.MkNot(pred)
		if err != nil {
			return false, err
		}
		if err = s.hold(notPred); err != nil {
			return false, err
		}
		first, err := m.MkOr(notPred, equality)
		if err != nil {
			return false, err
		}
		if err = nonlinear.AssertWithProof(first, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return false, err
		}
		notEquality, err := m.MkNot(equality)
		if err != nil {
			return false, err
		}
		if err = s.hold(notEquality); err != nil {
			return false, err
		}
		second, err := m.MkOr(pred, notEquality)
		if err != nil {
			return false, err
		}
		if err = nonlinear.AssertWithProof(second, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return false, err
		}
		iff, err := m.MkIff(pred, equality)
		if err != nil {
			return false, err
		}
		if err = smt.AssertFormula(iff); err != nil {
			return false, err
		}
		if err = s.hold(v); err != nil {
			return false, err
		}
		if err = s.hold(w); err != nil {
			return false, err
		}
		s.pairs[pair] = pred
		s.predicates = append(s.predicates, pred)
		s.values = append(s.values, lbool.True)
		changed = true
	}
	return changed, nil
}

func (s *nlEqualityState) Close() (result error) {
	if s == nil || s.manager == nil {
		return nil
	}
	for i := len(s.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, s.manager.DelRef(&s.refs[i]))
	}
	s.manager = nil
	s.refs = nil
	s.pairs = nil
	s.filter = nil
	return result
}
