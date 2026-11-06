package rel

// Source: check_relation wrapping udoc_relation. The expected formula is
// independent of the actual DOC representation and never drives execution.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

type udocFormulaChecker struct {
	*relationFormulaChecker
	formula ast.ASTRef
}

func (r *TypedUDocRelation) FormulaChecked() bool { return r != nil && r.checker != nil }

func (r *TypedUDocRelation) installChecker(expected ast.ASTHandle) error {
	if r.factory.checker == nil {
		return nil
	}
	r.checker = &udocFormulaChecker{relationFormulaChecker: &relationFormulaChecker{
		manager: r.factory.manager, sorts: r.sorts, config: *r.factory.checker,
	}}
	var err error
	r.checker.formula, err = r.factory.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		r.checker = nil
		return err
	}
	return r.checkFormula(expected)
}

func (r *TypedUDocRelation) checkFormula(expected ast.ASTHandle) (resultErr error) {
	actual, err := r.toUncheckedFormula()
	if err != nil {
		return err
	}
	m := r.factory.manager
	owner := &typedUDocFormulaOwner{manager: m, roots: []ast.ASTRef{actual}}
	defer func() { resultErr = finishTypedUDocFormula(owner, resultErr) }()
	got, err := m.RefGet(actual)
	if err != nil {
		return err
	}
	if err = r.checker.verifyGrounded(expected, got, false); err != nil {
		return fmt.Errorf("doc relation: %w", err)
	}
	return m.SetRef(r.checker.formula, got)
}

func (r *TypedUDocRelation) checkCurrentFormula() error {
	if r.checker == nil {
		return nil
	}
	expected, err := r.factory.manager.RefGet(r.checker.formula)
	if err != nil {
		return err
	}
	return r.checkFormula(expected)
}

func (r *TypedUDocRelation) checkedResult(result *TypedUDocRelation, expected ast.ASTRef, cause error) (output *TypedUDocRelation, resultErr error) {
	if r.checker == nil {
		return result, cause
	}
	if cause != nil {
		return nil, finishCheckedUDocConstruction(cause, result, expected)
	}
	formula, err := r.factory.manager.RefGet(expected)
	if err != nil {
		return nil, finishCheckedUDocConstruction(err, result, expected)
	}
	if err = result.installChecker(formula); err != nil {
		return nil, finishCheckedUDocConstruction(err, result, expected)
	}
	if err = finishCheckedUDocConstruction(nil, result, expected); err != nil {
		return nil, err
	}
	return result, nil
}

func finishCheckedUDocConstruction(cause error, result *TypedUDocRelation, expected ast.ASTRef) error {
	if result == nil {
		return cause
	}
	m := result.factory.manager
	if result.checker != nil {
		m = result.checker.manager
	}
	owner := &typedUDocFormulaOwner{manager: m, relation: result}
	if expected != (ast.ASTRef{}) {
		owner.roots = append(owner.roots, expected)
	}
	return finishTypedUDocFormula(owner, cause)
}

func (r *TypedUDocRelation) checkerFilter(condition ast.ASTHandle) (resultErr error) {
	if r.checker == nil {
		return nil
	}
	m := r.factory.manager
	owner := &typedUDocFormulaOwner{manager: m}
	defer func() { resultErr = finishTypedUDocFormula(owner, resultErr) }()
	old, err := m.RefGet(r.checker.formula)
	if err != nil {
		return err
	}
	expected, err := m.MkAndArgs([]ast.ASTHandle{old, condition})
	if err != nil {
		return err
	}
	if _, err = owner.hold(expected, nil); err != nil {
		return err
	}
	return r.checkFormula(expected)
}

func (r *TypedUDocRelation) checkerAddFact(values []ast.ASTHandle) (resultErr error) {
	if r.checker == nil {
		return nil
	}
	m := r.factory.manager
	owner := &typedUDocFormulaOwner{manager: m}
	defer func() { resultErr = finishTypedUDocFormula(owner, resultErr) }()
	fact, err := r.checker.factFormula(values)
	if fact != (ast.ASTRef{}) {
		owner.roots = append(owner.roots, fact)
	}
	if err != nil {
		return err
	}
	value, err := m.RefGet(fact)
	if err != nil {
		return err
	}
	old, err := m.RefGet(r.checker.formula)
	if err != nil {
		return err
	}
	expected, err := m.MkOrArgs([]ast.ASTHandle{old, value})
	if err != nil {
		return err
	}
	if _, err = owner.hold(expected, nil); err != nil {
		return err
	}
	return r.checkFormula(expected)
}

func (r *TypedUDocRelation) checkerContains(values []ast.ASTHandle, found bool) (resultErr error) {
	if r.checker == nil {
		return nil
	}
	m := r.factory.manager
	owner := &typedUDocFormulaOwner{manager: m}
	defer func() { resultErr = finishTypedUDocFormula(owner, resultErr) }()
	fact, err := r.checker.factFormula(values)
	if fact != (ast.ASTRef{}) {
		owner.roots = append(owner.roots, fact)
	}
	if err != nil {
		return err
	}
	point, err := m.RefGet(fact)
	if err != nil {
		return err
	}
	formula, err := m.RefGet(r.checker.formula)
	if err != nil {
		return err
	}
	intersection, err := owner.hold(m.MkAndArgs([]ast.ASTHandle{formula, point}))
	if err != nil {
		return err
	}
	if found {
		return r.checker.verifyGrounded(point, intersection, false)
	}
	falsity, err := owner.hold(m.MkFalse())
	if err != nil {
		return err
	}
	return r.checker.verifyGrounded(falsity, intersection, false)
}

func (r *TypedUDocRelation) checkerProject(removed []uint32) (ast.ASTRef, error) {
	if r.checker == nil {
		return ast.ASTRef{}, nil
	}
	m := r.factory.manager
	source, err := m.RefGet(r.checker.formula)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if len(removed) == 0 {
		return m.NewRef(source)
	}
	columns := make([]int, len(removed))
	for i, column := range removed {
		columns[i] = int(column)
	}
	return r.checker.projectFormulaForSorts(source, r.sorts, columns)
}

func (r *TypedUDocRelation) checkerJoin(other *TypedUDocRelation, left, right, removed []uint32) (result ast.ASTRef, resultErr error) {
	if r.checker == nil {
		return result, nil
	}
	if other.checker == nil {
		return result, ErrRelationVerification
	}
	joined, err := r.checker.joinFormulaRefs(r.checker.formula, other.checker.formula, r.sorts, other.sorts, left, right)
	if err != nil || len(removed) == 0 {
		return joined, err
	}
	m := r.factory.manager
	owner := &typedUDocFormulaOwner{manager: m, roots: []ast.ASTRef{joined}}
	defer func() {
		resultErr = finishTypedUDocFormula(owner, resultErr)
		if resultErr == nil {
			result, owner.result = owner.result, ast.ASTRef{}
		} else {
			result = ast.ASTRef{}
		}
	}()
	body, err := m.RefGet(joined)
	if err != nil {
		return result, err
	}
	sorts := append(append([]ast.ASTHandle(nil), r.sorts...), other.sorts...)
	columns := make([]int, len(removed))
	for i, column := range removed {
		columns[i] = int(column)
	}
	owner.result, resultErr = r.checker.projectFormulaForSorts(body, sorts, columns)
	return owner.result, resultErr
}

func (r *TypedUDocRelation) checkerFilterProject(condition ast.ASTHandle, removed []uint32) (result ast.ASTRef, resultErr error) {
	if r.checker == nil {
		return result, nil
	}
	m := r.factory.manager
	owner := &typedUDocFormulaOwner{manager: m}
	defer func() {
		resultErr = finishTypedUDocFormula(owner, resultErr)
		if resultErr == nil {
			result, owner.result = owner.result, ast.ASTRef{}
		} else {
			result = ast.ASTRef{}
		}
	}()
	source, err := m.RefGet(r.checker.formula)
	if err != nil {
		return result, err
	}
	filtered, err := owner.hold(m.MkAndArgs([]ast.ASTHandle{source, condition}))
	if err != nil {
		return result, err
	}
	if len(removed) == 0 {
		owner.result, err = m.NewRef(filtered)
		return result, err
	}
	columns := make([]int, len(removed))
	for i, column := range removed {
		columns[i] = int(column)
	}
	owner.result, err = r.checker.projectFormulaForSorts(filtered, r.sorts, columns)
	return result, err
}

func (r *TypedUDocRelation) checkerEqual(column uint32, value ast.ASTHandle) (resultErr error) {
	if r.checker == nil {
		return nil
	}
	m := r.factory.manager
	owner := &typedUDocFormulaOwner{manager: m}
	defer func() { resultErr = finishTypedUDocFormula(owner, resultErr) }()
	variable, err := owner.hold(m.MkVar(column, r.sorts[column]))
	if err != nil {
		return err
	}
	condition, err := owner.hold(m.MkEq(variable, value))
	if err != nil {
		return err
	}
	return r.checkerFilter(condition)
}

func (r *TypedUDocRelation) checkerIdentical(columns []uint32) (resultErr error) {
	if r.checker == nil {
		return nil
	}
	m := r.factory.manager
	owner := &typedUDocFormulaOwner{manager: m}
	defer func() { resultErr = finishTypedUDocFormula(owner, resultErr) }()
	first, err := owner.hold(m.MkVar(columns[0], r.sorts[columns[0]]))
	if err != nil {
		return err
	}
	var conditions []ast.ASTHandle
	for _, column := range columns[1:] {
		next, err := owner.hold(m.MkVar(column, r.sorts[column]))
		if err != nil {
			return err
		}
		eq, err := owner.hold(m.MkEq(first, next))
		if err != nil {
			return err
		}
		conditions = append(conditions, eq)
	}
	condition, err := owner.hold(m.MkAndArray(conditions))
	if err != nil {
		return err
	}
	return r.checkerFilter(condition)
}

func (r *TypedUDocRelation) checkerUnion(source ast.ASTRef, delta *TypedUDocRelation, old, deltaOld ast.ASTRef) (resultErr error) {
	m := r.factory.manager
	owner := &typedUDocFormulaOwner{manager: m}
	defer func() { resultErr = finishTypedUDocFormula(owner, resultErr) }()
	actual, err := r.toUncheckedFormula()
	if err != nil {
		return err
	}
	owner.roots = append(owner.roots, actual)
	before, err := m.RefGet(old)
	if err != nil {
		return err
	}
	input, err := m.RefGet(source)
	if err != nil {
		return err
	}
	after, err := m.RefGet(actual)
	if err != nil {
		return err
	}
	var previousDelta, currentDelta ast.ASTHandle
	if delta != nil {
		previousDelta, err = m.RefGet(deltaOld)
		if err != nil {
			return err
		}
		value, err := delta.toUncheckedFormula()
		if err != nil {
			return err
		}
		owner.roots = append(owner.roots, value)
		currentDelta, err = m.RefGet(value)
		if err != nil {
			return err
		}
	}
	if err = r.checker.verifyUnion(before, input, after, previousDelta, currentDelta, delta != nil); err != nil {
		return err
	}
	if err = m.SetRef(r.checker.formula, after); err != nil {
		return err
	}
	if delta != nil {
		return m.SetRef(delta.checker.formula, currentDelta)
	}
	return nil
}
