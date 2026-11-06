package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

func verifyIntervalRelationExpected(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, expected ast.ASTHandle, actual *IntervalRelation, named bool) (resultErr error) {
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
	return verifyRelationFormulaPair(manager, sorts, config, expected, actualHandle, named)
}

func VerifyIntervalRelationConstant(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, relation *IntervalRelation, full bool) error {
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
	return verifyIntervalRelationExpected(manager, sorts, config, expected, relation, false)
}

func VerifyIntervalRelationClone(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, source, clone *IntervalRelation) (resultErr error) {
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
	return verifyIntervalRelationExpected(manager, sorts, config, sourceHandle, clone, false)
}

func VerifyIntervalRelationEmpty(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, relation *IntervalRelation) error {
	if manager == nil || relation == nil {
		return ErrRelationVerification
	}
	if !relation.Empty() {
		return nil
	}
	expected, err := manager.MkFalse()
	if err != nil {
		return err
	}
	ref, err := manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer manager.DelRef(&ref)
	return verifyIntervalRelationExpected(manager, sorts, config, expected, relation, false)
}

// VerifyIntervalRelationFactAdd checks the source check_relation::add_fact
// postcondition against the interval relation's concrete formula.
func VerifyIntervalRelationFactAdd(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, before ast.ASTHandle, values []ast.ASTHandle, after *IntervalRelation) (resultErr error) {
	if manager == nil || before.IsNull() || len(sorts) != len(values) || after == nil {
		return ErrRelationVerification
	}
	checker := relationFormulaChecker{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...), config: config}
	fact, err := checker.factFormula(values)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&fact)) }()
	factHandle, err := manager.RefGet(fact)
	if err != nil {
		return err
	}
	expected, err := manager.MkOrArgs([]ast.ASTHandle{before, factHandle})
	if err != nil {
		return err
	}
	expectedRef, err := manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&expectedRef)) }()
	return verifyIntervalRelationExpected(manager, sorts, config, expected, after, false)
}

func VerifyIntervalRelationFilter(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, before, condition ast.ASTHandle, after *IntervalRelation, named bool) (resultErr error) {
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
	return verifyIntervalRelationExpected(manager, sorts, config, expected, after, named)
}

func VerifyIntervalRelationUnion(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, before ast.ASTHandle, source, after *IntervalRelation) (resultErr error) {
	if manager == nil || before.IsNull() || source == nil || after == nil {
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
	expected, err := manager.MkOrArgs([]ast.ASTHandle{before, sourceHandle})
	if err != nil {
		return err
	}
	expectedRef, err := manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&expectedRef)) }()
	return verifyIntervalRelationExpected(manager, sorts, config, expected, after, true)
}

func VerifyIntervalRelationJoin(manager *ast.ASTManager, leftSorts, rightSorts []ast.ASTHandle, config smtcore.SMTSolverConfig, left, right *IntervalRelation, leftColumns, rightColumns []uint32, after *IntervalRelation) (resultErr error) {
	if manager == nil || left == nil || right == nil || after == nil || len(leftColumns) != len(rightColumns) {
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
	resultSorts := append(append([]ast.ASTHandle(nil), leftSorts...), rightSorts...)
	return verifyIntervalRelationExpected(manager, resultSorts, config, expected, after, true)
}

func VerifyIntervalRelationProject(manager *ast.ASTManager, sourceSorts, resultSorts []ast.ASTHandle, config smtcore.SMTSolverConfig, source ast.ASTHandle, removed []int, after *IntervalRelation) error {
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
	return verifyIntervalRelationExpected(manager, resultSorts, config, handle, after, true)
}

func VerifyIntervalRelationRename(manager *ast.ASTManager, sourceSorts, resultSorts []ast.ASTHandle, config smtcore.SMTSolverConfig, source ast.ASTHandle, cycle []uint32, after *IntervalRelation) (resultErr error) {
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
	handle, err := manager.RefGet(expected)
	if err != nil {
		return err
	}
	return verifyIntervalRelationExpected(manager, resultSorts, config, handle, after, true)
}
