package rel

// Source: src/muz/rel/dl_table_relation.cpp operation-forwarding functors.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func tableRelationRemovedColumns(count int, removed []int) ([]bool, error) {
	flags := make([]bool, count)
	previous := -1
	for _, column := range removed {
		if column <= previous || column < 0 || column >= count {
			return nil, fmt.Errorf("%w: removed columns must be increasing and in range", ErrTableRelation)
		}
		flags[column] = true
		previous = column
	}
	return flags, nil
}

func tableRelationCyclePermutation(count int, cycle []int) ([]int, error) {
	permutation := make([]int, count)
	for i := range permutation {
		permutation[i] = i
	}
	if len(cycle) < 2 {
		return permutation, nil
	}
	seen := make([]bool, count)
	for _, column := range cycle {
		if column < 0 || column >= count || seen[column] {
			return nil, fmt.Errorf("%w: invalid rename cycle", ErrTableRelation)
		}
		seen[column] = true
	}
	for i := 1; i < len(cycle); i++ {
		permutation[cycle[i-1]] = cycle[i]
	}
	permutation[cycle[len(cycle)-1]] = cycle[0]
	return permutation, nil
}

func (r *TableRelation) compatible(other *TableRelation) bool {
	if r == nil || other == nil || r.manager != other.manager || len(r.sorts) != len(other.sorts) {
		return false
	}
	for i := range r.sorts {
		if r.sorts[i] != other.sorts[i] {
			return false
		}
	}
	return true
}

func (r *TableRelation) Join(other *TableRelation, leftColumns, rightColumns []int) (*TableRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if other == nil || other.open() != nil || other.manager != r.manager {
		return nil, ErrTableRelation
	}
	joined, err := r.table.Join(other.table, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	sorts := append(append([]ast.ASTHandle(nil), r.sorts...), other.sorts...)
	return AdoptTableRelation(r.manager, sorts, joined)
}

func (r *TableRelation) JoinProject(other *TableRelation, leftColumns, rightColumns, removed []int) (*TableRelation, error) {
	joined, err := r.Join(other, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	result, err := joined.Project(removed)
	closeErr := joined.Close()
	if err != nil {
		return nil, errors.Join(err, closeErr)
	}
	return result, closeErr
}

func (r *TableRelation) Project(removed []int) (*TableRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	projected, err := r.table.Project(removed)
	if err != nil {
		return nil, err
	}
	flags, err := tableRelationRemovedColumns(len(r.sorts), removed)
	if err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, 0, len(r.sorts)-len(removed))
	for i, sort := range r.sorts {
		if !flags[i] {
			sorts = append(sorts, sort)
		}
	}
	return AdoptTableRelation(r.manager, sorts, projected)
}

func (r *TableRelation) RenameCycle(cycle []int) (*TableRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	permutation, err := tableRelationCyclePermutation(len(r.sorts), cycle)
	if err != nil {
		return nil, err
	}
	return r.Permute(permutation)
}

func (r *TableRelation) Permute(permutation []int) (*TableRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	renamed, err := r.table.Permute(permutation)
	if err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, len(permutation))
	for i, source := range permutation {
		if source < 0 || source >= len(r.sorts) {
			return nil, ErrTableRelation
		}
		sorts[i] = r.sorts[source]
	}
	return AdoptTableRelation(r.manager, sorts, renamed)
}

func (r *TableRelation) SelectEqualAndProject(value ast.ASTHandle, column int) (*TableRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if column < 0 || column >= len(r.sorts) {
		return nil, ErrTableRelation
	}
	element, err := r.manager.EncodeValue(r.sorts[column], value)
	if err != nil {
		return nil, err
	}
	table, err := r.table.SelectEqualAndProject(element, column)
	if err != nil {
		return nil, err
	}
	sorts := append([]ast.ASTHandle(nil), r.sorts[:column]...)
	sorts = append(sorts, r.sorts[column+1:]...)
	return AdoptTableRelation(r.manager, sorts, table)
}

func (r *TableRelation) UnionFrom(source, delta *TableRelation) error {
	if err := r.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || !r.compatible(source) {
		return fmt.Errorf("%w: union signatures differ", ErrTableRelation)
	}
	var deltaTable *HashTable
	if delta != nil {
		if delta.open() != nil || !r.compatible(delta) {
			return fmt.Errorf("%w: delta signature differs", ErrTableRelation)
		}
		deltaTable = delta.table
	}
	return r.table.Union(source.table, deltaTable)
}

func (r *TableRelation) FilterEqual(value ast.ASTHandle, column int) error {
	if err := r.open(); err != nil {
		return err
	}
	if column < 0 || column >= len(r.sorts) {
		return ErrTableRelation
	}
	element, err := r.manager.EncodeValue(r.sorts[column], value)
	if err != nil {
		return err
	}
	return r.table.FilterEqual(element, column)
}

func (r *TableRelation) FilterIdentical(columns []int) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.table.FilterIdentical(columns)
}

func (r *TableRelation) FilterByNegation(source *TableRelation, targetColumns, sourceColumns []int) error {
	if err := r.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || source.manager != r.manager {
		return ErrTableRelation
	}
	return r.table.FilterByNegation(source.table, targetColumns, sourceColumns)
}
