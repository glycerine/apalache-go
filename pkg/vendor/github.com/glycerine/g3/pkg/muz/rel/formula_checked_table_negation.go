package rel

// Source: src/muz/rel/check_relation.cpp, verify_filter_by_negation.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// FilterByNegation removes target rows matching the negative relation and
// checks the resulting formula against the existential anti-join condition.
func (r *FormulaCheckedTable) FilterByNegation(negative *FormulaCheckedTable, columns, negativeColumns []int) (resultErr error) {
	if err := r.open(); err != nil {
		return err
	}
	if negative.open() != nil || r.manager != negative.manager || len(columns) != len(negativeColumns) {
		return ErrTableRelation
	}
	for i, column := range columns {
		other := negativeColumns[i]
		if column < 0 || column >= len(r.table.sorts) || other < 0 || other >= len(negative.table.sorts) || r.table.sorts[column] != negative.table.sorts[other] {
			return ast.ErrASTDomain
		}
	}
	m := r.manager
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	hold := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return h, nil
	}
	old, err := r.table.ToFormula()
	if err != nil {
		return err
	}
	refs = append(refs, old)
	oldHandle, err := m.RefGet(old)
	if err != nil {
		return err
	}
	neg, err := negative.table.ToFormula()
	if err != nil {
		return err
	}
	refs = append(refs, neg)
	negHandle, err := m.RefGet(neg)
	if err != nil {
		return err
	}
	terms := []ast.ASTHandle{negHandle}
	for i, column := range columns {
		other := negativeColumns[i]
		left, e := hold(m.MkVar(uint32(len(negative.table.sorts)+column), r.table.sorts[column]))
		if e != nil {
			return e
		}
		right, e := hold(m.MkVar(uint32(other), negative.table.sorts[other]))
		if e != nil {
			return e
		}
		equality, e := hold(m.MkEq(left, right))
		if e != nil {
			return e
		}
		terms = append(terms, equality)
	}
	body, err := hold(m.MkAndArray(terms))
	if err != nil {
		return err
	}
	bound := append([]ast.ASTHandle(nil), negative.table.sorts...)
	for i, j := 0, len(bound)-1; i < j; i, j = i+1, j-1 {
		bound[i], bound[j] = bound[j], bound[i]
	}
	names := make([]symbol.Symbol, len(bound))
	for i := range names {
		names[i] = symbol.Numerical(uint32(i))
	}
	// A nullary negative has no witnesses to bind, so its existential
	// condition is the body itself. The source AST factory requires at
	// least one binder (src/ast/ast.cpp, ast_manager::mk_quantifier).
	exists := body
	if len(bound) != 0 {
		exists, err = hold(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: bound, Names: names, Body: body}))
		if err != nil {
			return err
		}
	}
	notExists, err := hold(m.MkNot(exists))
	if err != nil {
		return err
	}
	expected, err := hold(m.MkAndArgs([]ast.ASTHandle{oldHandle, notExists}))
	if err != nil {
		return err
	}
	if err = r.table.FilterByNegation(negative.table, columns, negativeColumns); err != nil {
		return err
	}
	actual, err := r.table.ToFormula()
	if err != nil {
		return err
	}
	refs = append(refs, actual)
	actualHandle, err := m.RefGet(actual)
	if err != nil {
		return err
	}
	if err = m.SetRef(r.formula, actualHandle); err != nil {
		return err
	}
	if err = r.verifyGrounded(actualHandle, expected, false); err != nil {
		return fmt.Errorf("filter by negation: %w", err)
	}
	return nil
}
