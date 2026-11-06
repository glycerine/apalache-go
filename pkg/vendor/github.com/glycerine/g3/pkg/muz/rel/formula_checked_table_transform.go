package rel

// Source: src/muz/rel/check_relation.cpp, relation transformations.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (r *relationFormulaChecker) substitute(formula ast.ASTHandle, bindings []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	subst, err := rewriter.NewVarSubstitution(r.manager, false)
	if subst != nil {
		defer func() {
			resultErr = errors.Join(resultErr, subst.Close())
			if resultErr != nil && result != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, r.manager.DelRef(&result))
				result = ast.ASTRef{}
			}
		}()
	}
	if err != nil {
		return result, err
	}
	result, err = r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	resultErr = subst.Substitute(formula, bindings, result)
	return result, resultErr
}

func (r *FormulaCheckedTable) projectFormula(formula ast.ASTHandle, removed []int) (result ast.ASTRef, resultErr error) {
	return r.projectFormulaForSorts(formula, r.table.sorts, removed)
}

func (r *relationFormulaChecker) projectFormulaForSorts(formula ast.ASTHandle, sorts []ast.ASTHandle, removed []int) (result ast.ASTRef, resultErr error) {
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[i]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	bindings := make([]ast.ASTHandle, len(sorts))
	var bound []ast.ASTHandle
	var names []symbol.Symbol
	removedIndex, remainingIndex := 0, 0
	for i, sort := range sorts {
		index := remainingIndex + len(removed)
		if removedIndex < len(removed) && removed[removedIndex] == i {
			index = removedIndex
			bound = append(bound, sort)
			name, err := r.manager.Symbols.Intern(fmt.Sprintf("x%d", removedIndex))
			if err != nil {
				return result, err
			}
			names = append(names, name)
			removedIndex++
		} else {
			remainingIndex++
		}
		variable, err := r.manager.MkVar(uint32(index), sort)
		if err != nil {
			return result, err
		}
		ref, err := r.manager.NewRef(variable)
		if err != nil {
			return result, err
		}
		refs = append(refs, ref)
		bindings[i] = variable
	}
	if removedIndex != len(removed) {
		return result, ast.ErrASTDomain
	}
	body, err := r.substitute(formula, bindings)
	if err != nil {
		return result, err
	}
	refs = append(refs, body)
	bodyHandle, err := r.manager.RefGet(body)
	if err != nil {
		return result, err
	}
	for i, j := 0, len(bound)-1; i < j; i, j = i+1, j-1 {
		bound[i], bound[j] = bound[j], bound[i]
	}
	quantifier, err := r.manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: bound, Names: names, Body: bodyHandle})
	if err != nil {
		return result, err
	}
	return r.manager.NewRef(quantifier)
}

// Project verifies that the output formula is the existential projection of
// the source formula over the removed columns.
func (r *FormulaCheckedTable) Project(removed []int) (_ *FormulaCheckedTable, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if len(removed) == 0 {
		return nil, ast.ErrASTDomain
	}
	if _, err := tableRelationRemovedColumns(len(r.table.sorts), removed); err != nil {
		return nil, err
	}
	table, err := r.table.Project(removed)
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedTable(table, r.config)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	source, err := r.table.ToFormula()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&source)) }()
	sourceHandle, err := r.manager.RefGet(source)
	if err != nil {
		return nil, err
	}
	projected, err := r.projectFormula(sourceHandle, removed)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&projected)) }()
	expected, err := r.manager.RefGet(projected)
	if err != nil {
		return nil, err
	}
	actual, err := r.manager.RefGet(result.formula)
	if err != nil {
		return nil, err
	}
	if err = result.verifyGrounded(expected, actual, false); err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	return result, nil
}

// RenameCycle applies the concrete table rename and checks the source formula
// under the corresponding variable permutation.
func (r *FormulaCheckedTable) RenameCycle(cycle []int) (_ *FormulaCheckedTable, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if len(cycle) < 2 {
		return nil, ast.ErrASTDomain
	}
	if _, err := tableRelationCyclePermutation(len(r.table.sorts), cycle); err != nil {
		return nil, err
	}
	table, err := r.table.RenameCycle(cycle)
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedTable(table, r.config)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	permutation := make([]int, len(r.table.sorts))
	for i := range permutation {
		permutation[i] = i
	}
	for i, column := range cycle {
		permutation[cycle[(i+1)%len(cycle)]] = column
	}
	bindings := make([]ast.ASTHandle, len(permutation))
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[i]))
		}
	}()
	for i, dest := range permutation {
		if result.table.sorts[dest] != r.table.sorts[i] {
			return nil, ast.ErrASTDomain
		}
		variable, err := r.manager.MkVar(uint32(dest), r.table.sorts[i])
		if err != nil {
			return nil, err
		}
		ref, err := r.manager.NewRef(variable)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
		bindings[i] = variable
	}
	sourceFormula, err := r.table.ToFormula()
	if err != nil {
		return nil, err
	}
	refs = append(refs, sourceFormula)
	source, err := r.manager.RefGet(sourceFormula)
	if err != nil {
		return nil, err
	}
	permuted, err := r.substitute(source, bindings)
	if err != nil {
		return nil, err
	}
	refs = append(refs, permuted)
	want, err := r.manager.RefGet(permuted)
	if err != nil {
		return nil, err
	}
	got, err := r.manager.RefGet(result.formula)
	if err != nil {
		return nil, err
	}
	if err = result.verifyGrounded(want, got, false); err != nil {
		return nil, fmt.Errorf("permutation: %w", err)
	}
	return result, nil
}

// FilterInterpretedAndProject checks the source's fused filter and projection.
func (r *FormulaCheckedTable) FilterInterpretedAndProject(theory *rewriter.TheoryRewriter, condition ast.ASTHandle, removed []int) (_ *FormulaCheckedTable, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if len(removed) == 0 {
		return nil, ast.ErrASTDomain
	}
	if _, err := tableRelationRemovedColumns(len(r.table.sorts), removed); err != nil {
		return nil, err
	}
	columns, sorts, err := r.interpretedVariables(condition)
	if err != nil {
		return nil, err
	}
	table, err := r.table.FilterInterpretedAndProject(theory, []ast.ASTHandle{condition}, columns, sorts, removed)
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedTable(table, r.config)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	source, err := r.table.ToFormula()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&source)) }()
	sourceHandle, err := r.manager.RefGet(source)
	if err != nil {
		return nil, err
	}
	filtered, err := r.manager.MkAndArgs([]ast.ASTHandle{condition, sourceHandle})
	if err != nil {
		return nil, err
	}
	filteredRef, err := r.manager.NewRef(filtered)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&filteredRef)) }()
	projected, err := r.projectFormula(filtered, removed)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&projected)) }()
	expected, err := r.manager.RefGet(projected)
	if err != nil {
		return nil, err
	}
	actual, err := r.manager.RefGet(result.formula)
	if err != nil {
		return nil, err
	}
	if err = result.verifyGrounded(expected, actual, false); err != nil {
		return nil, fmt.Errorf("filter_project: %w", err)
	}
	return result, nil
}
