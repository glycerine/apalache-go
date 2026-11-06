package rel

// Source: src/muz/rel/check_relation.cpp, verify_union and union_fn.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *relationFormulaChecker) verifyUnion(old, source, actual, deltaOld, deltaActual ast.ASTHandle, hasDelta bool) (resultErr error) {
	m := r.manager
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	hold := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return value, nil
	}
	ground := func(value ast.ASTHandle) (ast.ASTHandle, error) {
		ref, err := r.groundWithNames(value, true)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return m.RefGet(ref)
	}
	contains := func(container, subset ast.ASTHandle) error {
		conjunction, err := hold(m.MkAndArgs([]ast.ASTHandle{container, subset}))
		if err != nil {
			return err
		}
		return r.checkEquivalent(conjunction, subset)
	}
	expected, err := hold(m.MkOrArgs([]ast.ASTHandle{source, old}))
	if err != nil {
		return err
	}
	expected, err = ground(expected)
	if err != nil {
		return err
	}
	actual, err = ground(actual)
	if err != nil {
		return err
	}
	if err = r.checkEquivalent(expected, actual); err != nil {
		return fmt.Errorf("union: %w", err)
	}
	if !hasDelta {
		return nil
	}
	notOld, err := hold(m.MkNot(old))
	if err != nil {
		return err
	}
	newDifference, err := hold(m.MkAndArgs([]ast.ASTHandle{actual, notOld}))
	if err != nil {
		return err
	}
	newDifference, err = ground(newDifference)
	if err != nil {
		return err
	}
	delta, err := ground(deltaActual)
	if err != nil {
		return err
	}
	if err = contains(delta, newDifference); err != nil {
		return fmt.Errorf("union delta low: %w", err)
	}
	previousDelta, err := ground(deltaOld)
	if err != nil {
		return err
	}
	if err = contains(delta, previousDelta); err != nil {
		return fmt.Errorf("union delta0: %w", err)
	}
	upperLeft, err := hold(m.MkOrArgs([]ast.ASTHandle{actual, deltaOld}))
	if err != nil {
		return err
	}
	upperRight, err := hold(m.MkOrArgs([]ast.ASTHandle{delta, old}))
	if err != nil {
		return err
	}
	upperLeft, err = ground(upperLeft)
	if err != nil {
		return err
	}
	upperRight, err = ground(upperRight)
	if err != nil {
		return err
	}
	if err = r.checkEquivalent(upperLeft, upperRight); err != nil {
		return fmt.Errorf("union no overflow: %w", err)
	}
	return nil
}

// UnionFrom applies and verifies a concrete table union, including all source
// delta checks when a distinct delta relation is provided.
func (r *FormulaCheckedTable) UnionFrom(source, delta *FormulaCheckedTable) (resultErr error) {
	if err := r.open(); err != nil {
		return err
	}
	if source.open() != nil || r.manager != source.manager || !r.table.compatible(source.table) {
		return ErrTableRelation
	}
	if delta != nil && (delta.open() != nil || delta == r || delta == source || r.manager != delta.manager || !r.table.compatible(delta.table)) {
		return ErrTableRelation
	}
	old, err := r.manager.CopyRef(r.formula)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&old)) }()
	var deltaOld ast.ASTRef
	if delta != nil {
		deltaOld, err = r.manager.CopyRef(delta.formula)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&deltaOld)) }()
	}
	var deltaTable *TableRelation
	if delta != nil {
		deltaTable = delta.table
	}
	if err = r.table.UnionFrom(source.table, deltaTable); err != nil {
		return err
	}
	actual, err := r.table.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&actual)) }()
	src, err := source.table.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&src)) }()
	var deltaActual ast.ASTRef
	if delta != nil {
		deltaActual, err = delta.table.ToFormula()
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&deltaActual)) }()
	}
	oldHandle, err := r.manager.RefGet(old)
	if err != nil {
		return err
	}
	srcHandle, err := r.manager.RefGet(src)
	if err != nil {
		return err
	}
	actualHandle, err := r.manager.RefGet(actual)
	if err != nil {
		return err
	}
	var oldDeltaHandle, deltaHandle ast.ASTHandle
	if delta != nil {
		oldDeltaHandle, err = r.manager.RefGet(deltaOld)
		if err != nil {
			return err
		}
		deltaHandle, err = r.manager.RefGet(deltaActual)
		if err != nil {
			return err
		}
	}
	if err = r.verifyUnion(oldHandle, srcHandle, actualHandle, oldDeltaHandle, deltaHandle, delta != nil); err != nil {
		return err
	}
	if err = r.manager.SetRef(r.formula, actualHandle); err != nil {
		return err
	}
	if delta != nil {
		return r.manager.SetRef(delta.formula, deltaHandle)
	}
	return nil
}
