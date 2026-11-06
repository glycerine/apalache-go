package tab

// Source: src/muz/tab/tab_context.cpp, index::check_substitution. The solver
// borrows the query precondition in an enclosing scope.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func validateSubsumption(m *ast.ASTManager, candidate *Clause, bindings *substitution.Substitution, side []ast.ASTRef, lite *qe.Lite, smt solver.Solver, falseLiterals map[ast.ASTHandle]ast.ASTRef, evaluator *model.ModelEvaluatorConfig) (valid bool, err error) {
	if m == nil || candidate == nil || candidate.closed || candidate.manager != m || bindings == nil || lite == nil || smt == nil {
		return false, ast.ErrASTHandle
	}
	solverManager, err := smt.Manager()
	if err != nil {
		return false, err
	}
	if solverManager != m {
		return false, ast.ErrASTHandle
	}
	attempt := &subsumptionCleanupAttempt{manager: m, scope: subsumptionSolverScope{solver: smt}}
	defer func() {
		err = finishSubsumptionScope(attempt, err)
		if err != nil {
			valid = false
		}
	}()
	if err = bindings.ResetCache(); err != nil {
		return false, err
	}
	apply := func(h ast.ASTHandle) error {
		ref, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		attempt.forms = append(attempt.forms, ref)
		return bindings.Apply(2, []uint32{0, 0}, substitution.NewExprOffset(h, 0), ref)
	}
	for _, ref := range side {
		h, e := m.RefGet(ref)
		if e != nil {
			return false, e
		}
		if e = apply(h); e != nil {
			return false, e
		}
	}
	constraint, err := candidate.Constraint()
	if err != nil {
		return false, err
	}
	if err = apply(constraint); err != nil {
		return false, err
	}
	if err = lite.EliminateConjunctions(nil, false, &attempt.forms); err != nil {
		return false, err
	}
	values := make([]ast.ASTHandle, len(attempt.forms))
	for i, ref := range attempt.forms {
		values[i], err = m.RefGet(ref)
		if err != nil {
			return false, err
		}
	}
	values, err = m.FlattenAnd(values)
	if err != nil {
		return false, err
	}
	for _, value := range values {
		key, e := normalizeSubsumptionLiteral(m, value)
		if e != nil {
			return false, e
		}
		if _, seen := falseLiterals[key]; seen {
			return false, nil
		}
	}
	attempt.post, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, values, attempt.post); err != nil {
		return false, err
	}
	h, err := m.RefGet(attempt.post)
	if err != nil {
		return false, err
	}
	if m.IsFalse(h) {
		return false, nil
	}
	if m.IsTrue(h) {
		return true, nil
	}
	ground, err := m.IsGround(h)
	if err != nil || !ground {
		return false, err
	}
	negated, err := m.MkNot(h)
	if err != nil {
		return false, err
	}
	if err = m.SetRef(attempt.post, negated); err != nil {
		return false, err
	}
	if err = attempt.scope.begin(); err != nil {
		return false, err
	}
	if err = smt.Assert(negated); err != nil {
		return false, err
	}
	status, err := smt.CheckSat(nil)
	if err != nil {
		return false, err
	}
	if status == lbool.True && falseLiterals != nil && evaluator != nil {
		if err = cacheFalseSubsumptionLiterals(m, smt, values, falseLiterals, *evaluator); err != nil {
			return false, err
		}
	}
	return status == lbool.False, nil
}

func normalizeSubsumptionLiteral(m *ast.ASTManager, h ast.ASTHandle) (ast.ASTHandle, error) {
	// Source is_eq accepts arbitrary expressions; non-applications pass
	// through unchanged before check_substitution's groundness restriction.
	nodeKind, err := m.GetKind(h)
	if err != nil || nodeKind != ast.ASTApp {
		return h, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != 0 {
		return h, err
	}
	kind, err := m.GetDeclKind(h)
	if err != nil || kind != ast.ASTOpEq {
		return h, err
	}
	count, err := m.GetNumArgs(h)
	if err != nil || count != 2 {
		return h, err
	}
	x, err := m.GetArg(h, 0)
	if err != nil {
		return h, err
	}
	y, err := m.GetArg(h, 1)
	if err != nil {
		return h, err
	}
	xID, err := m.GetASTID(x)
	if err != nil {
		return h, err
	}
	yID, err := m.GetASTID(y)
	if err != nil || xID <= yID {
		return h, err
	}
	return m.MkEq(y, x)
}
