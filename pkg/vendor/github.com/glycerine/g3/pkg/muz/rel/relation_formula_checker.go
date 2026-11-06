package rel

// Source: check_relation_plugin symbolic verification. The oracle depends on
// original column sorts and solver services, independently of relation storage.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type relationFormulaChecker struct {
	manager *ast.ASTManager
	sorts   []ast.ASTHandle
	config  smtcore.SMTSolverConfig
}

// GroundRelationFormula substitutes the source plugin checker's x0, x1, ...
// column constants for relation variables and returns an owned AST ref.
func GroundRelationFormula(manager *ast.ASTManager, sorts []ast.ASTHandle, formula ast.ASTHandle) (ast.ASTRef, error) {
	if manager == nil || formula.IsNull() {
		return ast.ASTRef{}, ErrRelationVerification
	}
	checker := &relationFormulaChecker{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...)}
	return checker.groundWithNames(formula, true)
}

// VerifyRelationFormulasEquivalent mirrors check_relation_plugin::check_equiv
// for formulas already grounded over the relation's named columns. Source
// treats unknown as inconclusive and rejects only a satisfiable difference.
func VerifyRelationFormulasEquivalent(manager *ast.ASTManager, config smtcore.SMTSolverConfig, left, right ast.ASTHandle) (resultErr error) {
	if manager == nil || left.IsNull() || right.IsNull() {
		return ErrRelationVerification
	}
	equivalence, err := manager.MkEq(left, right)
	if err != nil {
		return err
	}
	equivalenceRef, err := manager.NewRef(equivalence)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&equivalenceRef)) }()
	difference, err := manager.MkNot(equivalence)
	if err != nil {
		return err
	}
	differenceRef, err := manager.NewRef(difference)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&differenceRef)) }()
	solver, err := smtcore.NewSMTSolver(manager, nil, "", config)
	if solver != nil {
		defer func() { resultErr = errors.Join(resultErr, solver.Close()) }()
	}
	if err != nil {
		return err
	}
	if err = solver.Assert(difference); err != nil {
		return err
	}
	status, err := solver.CheckSat(nil)
	if err != nil {
		return err
	}
	if status == lbool.True {
		return ErrRelationVerification
	}
	return nil
}

// verifyRelationUnionDelta mirrors check_relation_plugin::verify_union's
// destination, delta coverage, and no-overflow postconditions.
func verifyRelationUnionDelta(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, dstBefore, source, dstAfter, deltaBefore, deltaAfter ast.ASTHandle) (resultErr error) {
	if manager == nil || dstBefore.IsNull() || source.IsNull() || dstAfter.IsNull() || deltaBefore.IsNull() || deltaAfter.IsNull() {
		return ErrRelationVerification
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.DelRef(&refs[i]))
		}
	}()
	hold := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := manager.NewRef(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return value, nil
	}
	ground := func(value ast.ASTHandle) (ast.ASTHandle, error) {
		ref, err := GroundRelationFormula(manager, sorts, value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return manager.RefGet(ref)
	}
	d0, err := ground(dstBefore)
	if err != nil {
		return err
	}
	s, err := ground(source)
	if err != nil {
		return err
	}
	dst, err := ground(dstAfter)
	if err != nil {
		return err
	}
	delta0, err := ground(deltaBefore)
	if err != nil {
		return err
	}
	delta, err := ground(deltaAfter)
	if err != nil {
		return err
	}
	union, err := hold(manager.MkOrArgs([]ast.ASTHandle{d0, s}))
	if err != nil {
		return err
	}
	if err = VerifyRelationFormulasEquivalent(manager, config, union, dst); err != nil {
		return err
	}
	notBefore, err := hold(manager.MkNot(d0))
	if err != nil {
		return err
	}
	newFacts, err := hold(manager.MkAndArgs([]ast.ASTHandle{dst, notBefore}))
	if err != nil {
		return err
	}
	coveredNewFacts, err := hold(manager.MkAndArgs([]ast.ASTHandle{delta, newFacts}))
	if err != nil {
		return err
	}
	if err = VerifyRelationFormulasEquivalent(manager, config, coveredNewFacts, newFacts); err != nil {
		return err
	}
	coveredOldDelta, err := hold(manager.MkAndArgs([]ast.ASTHandle{delta, delta0}))
	if err != nil {
		return err
	}
	if err = VerifyRelationFormulasEquivalent(manager, config, coveredOldDelta, delta0); err != nil {
		return err
	}
	left, err := hold(manager.MkOrArgs([]ast.ASTHandle{dst, delta0}))
	if err != nil {
		return err
	}
	right, err := hold(manager.MkOrArgs([]ast.ASTHandle{delta, d0}))
	if err != nil {
		return err
	}
	return VerifyRelationFormulasEquivalent(manager, config, left, right)
}

func verifyRelationFormulaPair(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, left, right ast.ASTHandle, named bool) (resultErr error) {
	if manager == nil || left.IsNull() || right.IsNull() {
		return ErrRelationVerification
	}
	checker := relationFormulaChecker{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...), config: config}
	groundLeft, err := checker.groundWithNames(left, named)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&groundLeft)) }()
	groundRight, err := checker.groundWithNames(right, named)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&groundRight)) }()
	leftHandle, err := manager.RefGet(groundLeft)
	if err != nil {
		return err
	}
	rightHandle, err := manager.RefGet(groundRight)
	if err != nil {
		return err
	}
	return VerifyRelationFormulasEquivalent(manager, config, leftHandle, rightHandle)
}
