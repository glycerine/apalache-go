package rel

// Source: src/muz/rel/dl_bound_relation.cpp bound_relation::to_formula.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
)

// ToFormula builds the source conjunction over column variables. The returned
// AST reference is owned by the caller.
func (r *BoundRelation) ToFormula(m *ast.ASTManager, sorts []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if r == nil || m == nil || len(sorts) != len(r.columns) {
		return result, ast.ErrASTDomain
	}
	var roots []ast.ASTRef
	owned := false
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&roots[i]))
		}
		if resultErr != nil && owned {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	variables := make([]ast.ASTHandle, len(sorts))
	for i, sort := range sorts {
		variable, err := m.MkVar(uint32(i), sort)
		if err != nil {
			return result, err
		}
		ref, err := m.NewRef(variable)
		if err != nil {
			return result, err
		}
		roots = append(roots, ref)
		variables[i] = variable
	}
	var conjuncts []ast.ASTHandle
	add := func(h ast.ASTHandle, err error) error {
		if err != nil {
			return err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return err
		}
		roots = append(roots, ref)
		conjuncts = append(conjuncts, h)
		return nil
	}
	for i := range r.columns {
		root := r.find(uint32(i))
		if root != uint32(i) {
			if err := add(m.MkEq(variables[i], variables[root])); err != nil {
				return result, err
			}
			continue
		}
		strict, weak, err := r.Bounds(uint32(i))
		if err != nil {
			return result, err
		}
		for _, j := range strict {
			if uint64(j) >= uint64(len(variables)) {
				return result, ast.ErrASTDomain
			}
			if err = add(m.MkArithmeticLT(variables[i], variables[j])); err != nil {
				return result, err
			}
		}
		for _, j := range weak {
			if uint64(j) >= uint64(len(variables)) {
				return result, ast.ErrASTDomain
			}
			if err = add(m.MkArithmeticLE(variables[i], variables[j])); err != nil {
				return result, err
			}
		}
	}
	result, resultErr = m.NewRef(ast.ASTHandle{})
	if resultErr != nil {
		return result, resultErr
	}
	owned = true
	resultErr = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, conjuncts, result)
	return result, resultErr
}

// VerifyBoundRelationUnion mirrors check_relation_plugin::verify_union for
// the bound relation backing. The union is exact only when the bound plugin's
// post-state formula is equivalent to the disjunction of its old state and
// the source relation.
func VerifyBoundRelationUnion(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, before ast.ASTHandle, source, after *BoundRelation) (resultErr error) {
	if manager == nil || before.IsNull() || source == nil || after == nil {
		return ErrRelationVerification
	}
	sourceFormula, err := source.ToFormula(manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&sourceFormula)) }()
	actualFormula, err := after.ToFormula(manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&actualFormula)) }()
	old := before
	added, err := manager.RefGet(sourceFormula)
	if err != nil {
		return err
	}
	expected, err := manager.MkOrArgs([]ast.ASTHandle{old, added})
	if err != nil {
		return err
	}
	expectedRef, err := manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&expectedRef)) }()
	actual, err := manager.RefGet(actualFormula)
	if err != nil {
		return err
	}
	groundExpected, err := GroundRelationFormula(manager, sorts, expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&groundExpected)) }()
	groundActual, err := GroundRelationFormula(manager, sorts, actual)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&groundActual)) }()
	expectedGround, err := manager.RefGet(groundExpected)
	if err != nil {
		return err
	}
	actualGround, err := manager.RefGet(groundActual)
	if err != nil {
		return err
	}
	return VerifyRelationFormulasEquivalent(manager, config, expectedGround, actualGround)
}

// VerifyBoundRelationFactAdd mirrors check_relation::add_fact: the expected
// post-state is the prior formula disjoined with the concrete tuple equation.
func VerifyBoundRelationFactAdd(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, before ast.ASTHandle, values []ast.ASTHandle, after *BoundRelation) (resultErr error) {
	if manager == nil || before.IsNull() || after == nil || len(sorts) != len(values) {
		return ErrRelationVerification
	}
	vars := make([]ast.ASTHandle, len(sorts))
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.DelRef(&roots[i]))
		}
	}()
	for i, sort := range sorts {
		v, err := manager.MkVar(uint32(i), sort)
		if err != nil {
			return err
		}
		ref, err := manager.NewRef(v)
		if err != nil {
			return err
		}
		roots = append(roots, ref)
		vars[i] = v
	}
	equalities := make([]ast.ASTHandle, len(values))
	for i, value := range values {
		eq, err := manager.MkEq(vars[i], value)
		if err != nil {
			return err
		}
		ref, err := manager.NewRef(eq)
		if err != nil {
			return err
		}
		roots = append(roots, ref)
		equalities[i] = eq
	}
	fact, err := manager.MkAndArgs(equalities)
	if err != nil {
		return err
	}
	factRef, err := manager.NewRef(fact)
	if err != nil {
		return err
	}
	roots = append(roots, factRef)
	expected, err := manager.MkOrArgs([]ast.ASTHandle{before, fact})
	if err != nil {
		return err
	}
	expectedRef, err := manager.NewRef(expected)
	if err != nil {
		return err
	}
	roots = append(roots, expectedRef)
	actualFormula, err := after.ToFormula(manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&actualFormula)) }()
	actual, err := manager.RefGet(actualFormula)
	if err != nil {
		return err
	}
	expectedGround, err := GroundRelationFormula(manager, sorts, expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&expectedGround)) }()
	actualGround, err := GroundRelationFormula(manager, sorts, actual)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&actualGround)) }()
	expectedHandle, err := manager.RefGet(expectedGround)
	if err != nil {
		return err
	}
	actualHandle, err := manager.RefGet(actualGround)
	if err != nil {
		return err
	}
	return VerifyRelationFormulasEquivalent(manager, config, expectedHandle, actualHandle)
}

// VerifyBoundRelationFilter mirrors check_relation_plugin::verify_filter for
// an in-place filter that should leave old_formula AND condition.
func VerifyBoundRelationFilter(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, before, condition ast.ASTHandle, after *BoundRelation, namedColumns bool) (resultErr error) {
	if manager == nil || before.IsNull() || condition.IsNull() || after == nil {
		return ErrRelationVerification
	}
	expected, err := manager.MkAndArgs([]ast.ASTHandle{before, condition})
	if err != nil {
		return err
	}
	expectedRef, err := manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&expectedRef)) }()
	actualFormula, err := after.ToFormula(manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&actualFormula)) }()
	actual, err := manager.RefGet(actualFormula)
	if err != nil {
		return err
	}
	checker := relationFormulaChecker{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...), config: config}
	ground := func(formula ast.ASTHandle) (ast.ASTRef, error) {
		return checker.groundWithNames(formula, namedColumns)
	}
	groundExpected, err := ground(expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&groundExpected)) }()
	groundActual, err := ground(actual)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&groundActual)) }()
	expectedHandle, err := manager.RefGet(groundExpected)
	if err != nil {
		return err
	}
	actualHandle, err := manager.RefGet(groundActual)
	if err != nil {
		return err
	}
	return VerifyRelationFormulasEquivalent(manager, config, expectedHandle, actualHandle)
}

func verifyBoundExpected(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, expected ast.ASTHandle, actual *BoundRelation) (resultErr error) {
	if manager == nil || expected.IsNull() || actual == nil {
		return ErrRelationVerification
	}
	actualFormula, err := actual.ToFormula(manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&actualFormula)) }()
	actualHandle, err := manager.RefGet(actualFormula)
	if err != nil {
		return err
	}
	groundExpected, err := GroundRelationFormula(manager, sorts, expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&groundExpected)) }()
	groundActual, err := GroundRelationFormula(manager, sorts, actualHandle)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&groundActual)) }()
	expectedHandle, err := manager.RefGet(groundExpected)
	if err != nil {
		return err
	}
	actualHandle, err = manager.RefGet(groundActual)
	if err != nil {
		return err
	}
	return VerifyRelationFormulasEquivalent(manager, config, expectedHandle, actualHandle)
}

// VerifyBoundRelationConstant checks the formulas produced by mk_empty and
// mk_full in check_relation_plugin.
func VerifyBoundRelationConstant(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, relation *BoundRelation, full bool) error {
	if manager == nil || relation == nil {
		return ErrRelationVerification
	}
	var expected ast.ASTHandle
	var err error
	if full {
		expected, err = manager.MkTrue()
	} else {
		expected, err = manager.MkFalse()
	}
	if err != nil {
		return err
	}
	ref, err := manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer manager.DelRef(&ref)
	return verifyBoundExpected(manager, sorts, config, expected, relation)
}

// VerifyBoundRelationClone mirrors check_relation::clone's source/result
// formula comparison.
func VerifyBoundRelationClone(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, source, clone *BoundRelation) (resultErr error) {
	if manager == nil || source == nil || clone == nil {
		return ErrRelationVerification
	}
	sourceFormula, err := source.ToFormula(manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&sourceFormula)) }()
	sourceHandle, err := manager.RefGet(sourceFormula)
	if err != nil {
		return err
	}
	return verifyBoundExpected(manager, sorts, config, sourceHandle, clone)
}

// VerifyBoundRelationJoin mirrors check_relation_plugin::verify_join.
func VerifyBoundRelationJoin(manager *ast.ASTManager, leftSorts, rightSorts []ast.ASTHandle, config smtcore.SMTSolverConfig, left, right *BoundRelation, leftColumns, rightColumns []uint32, after *BoundRelation) (resultErr error) {
	if manager == nil || left == nil || right == nil || len(leftColumns) != len(rightColumns) {
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
		if err == nil {
			refs = append(refs, ref)
		}
		return value, err
	}
	leftFormula, err := left.ToFormula(manager, leftSorts)
	if err != nil {
		return err
	}
	refs = append(refs, leftFormula)
	rightFormula, err := right.ToFormula(manager, rightSorts)
	if err != nil {
		return err
	}
	refs = append(refs, rightFormula)
	leftHandle, err := manager.RefGet(leftFormula)
	if err != nil {
		return err
	}
	rightHandle, err := manager.RefGet(rightFormula)
	if err != nil {
		return err
	}
	bindings := make([]ast.ASTHandle, len(rightSorts))
	for i, sort := range rightSorts {
		bindings[i], err = hold(manager.MkVar(uint32(len(leftSorts)+i), sort))
		if err != nil {
			return err
		}
	}
	checker := relationFormulaChecker{manager: manager}
	shifted, err := checker.substitute(rightHandle, bindings)
	if err != nil {
		return err
	}
	refs = append(refs, shifted)
	shiftedHandle, err := manager.RefGet(shifted)
	if err != nil {
		return err
	}
	expected, err := hold(manager.MkAndArgs([]ast.ASTHandle{leftHandle, shiftedHandle}))
	if err != nil {
		return err
	}
	for i, leftColumn := range leftColumns {
		rightColumn := rightColumns[i]
		if uint64(leftColumn) >= uint64(len(leftSorts)) || uint64(rightColumn) >= uint64(len(rightSorts)) || leftSorts[leftColumn] != rightSorts[rightColumn] {
			return ast.ErrASTDomain
		}
		leftVar, e := hold(manager.MkVar(leftColumn, leftSorts[leftColumn]))
		if e != nil {
			return e
		}
		rightVar, e := hold(manager.MkVar(uint32(len(leftSorts))+rightColumn, rightSorts[rightColumn]))
		if e != nil {
			return e
		}
		equality, e := hold(manager.MkEq(leftVar, rightVar))
		if e != nil {
			return e
		}
		expected, err = hold(manager.MkAndArgs([]ast.ASTHandle{equality, expected}))
		if err != nil {
			return err
		}
	}
	return verifyBoundExpected(manager, append(append([]ast.ASTHandle(nil), leftSorts...), rightSorts...), config, expected, after)
}

// VerifyBoundRelationProject checks that projection existentially quantifies
// exactly the removed columns, as check_relation_plugin::mk_project does.
func VerifyBoundRelationProject(manager *ast.ASTManager, sourceSorts, resultSorts []ast.ASTHandle, config smtcore.SMTSolverConfig, source ast.ASTHandle, removed []int, after *BoundRelation) error {
	checker := relationFormulaChecker{manager: manager}
	expected, err := checker.projectFormulaForSorts(source, sourceSorts, removed)
	if err != nil {
		return err
	}
	defer manager.DelRef(&expected)
	handle, err := manager.RefGet(expected)
	if err != nil {
		return err
	}
	return verifyBoundExpected(manager, resultSorts, config, handle, after)
}

// VerifyBoundRelationRename checks the source column permutation.
func VerifyBoundRelationRename(manager *ast.ASTManager, sourceSorts, resultSorts []ast.ASTHandle, config smtcore.SMTSolverConfig, source ast.ASTHandle, cycle []uint32, after *BoundRelation) (resultErr error) {
	if manager == nil || source.IsNull() || len(sourceSorts) != len(resultSorts) || len(cycle) < 2 {
		return ErrRelationVerification
	}
	permutation := make([]int, len(sourceSorts))
	for i := range permutation {
		permutation[i] = i
	}
	for i, column := range cycle {
		next := cycle[(i+1)%len(cycle)]
		if uint64(column) >= uint64(len(permutation)) || uint64(next) >= uint64(len(permutation)) {
			return ast.ErrASTDomain
		}
		permutation[next] = int(column)
	}
	checker := relationFormulaChecker{manager: manager}
	bindings := make([]ast.ASTHandle, len(sourceSorts))
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.DelRef(&refs[i]))
		}
	}()
	for i, destination := range permutation {
		if resultSorts[destination] != sourceSorts[i] {
			return ast.ErrASTDomain
		}
		variable, err := manager.MkVar(uint32(destination), sourceSorts[i])
		if err != nil {
			return err
		}
		ref, err := manager.NewRef(variable)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		bindings[i] = variable
	}
	expected, err := checker.substitute(source, bindings)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&expected)) }()
	expectedHandle, err := manager.RefGet(expected)
	if err != nil {
		return err
	}
	return verifyBoundExpected(manager, resultSorts, config, expectedHandle, after)
}
