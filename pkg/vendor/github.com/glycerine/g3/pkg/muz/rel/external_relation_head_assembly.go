package rel

// Source: src/muz/rel/dl_compiler.cpp, make_assembling_code,
// make_add_constant_column, and make_duplicate_column, specialized to the
// concrete external-relation operations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type externalHeadColumn struct {
	source   uint32
	constant ast.ASTHandle
}

func externalSourceHeadColumn(column uint32) externalHeadColumn {
	return externalHeadColumn{source: column}
}

func externalConstantHeadColumn(value ast.ASTHandle) externalHeadColumn {
	return externalHeadColumn{constant: value}
}

// assembleExternalRuleHead returns a fresh relation with source columns
// projected, duplicated, extended by preflight-created constant singletons,
// and renamed into head order. It never consumes source or the singletons.
func assembleExternalRuleHead(source *ExternalRelation, columns []externalHeadColumn, constants map[ast.ASTHandle]*ExternalRelation) (result *ExternalRelation, resultErr error) {
	if source == nil || source.open() != nil {
		return nil, ErrTableRelation
	}
	sorts, err := source.Sorts()
	if err != nil {
		return nil, err
	}
	referenced := make([]bool, len(sorts))
	for _, column := range columns {
		if !column.constant.IsNull() {
			isValue, cause := source.ast.IsValue(column.constant)
			if cause != nil || !isValue {
				return nil, errors.Join(cause, ErrCompilerUnsupported)
			}
			singleton := constants[column.constant]
			if singleton == nil || singleton.open() != nil || singleton.plugin != source.plugin {
				return nil, ErrTableRelation
			}
			singletonSorts, cause := singleton.Sorts()
			if cause != nil || len(singletonSorts) != 1 {
				return nil, errors.Join(cause, ErrTableRelation)
			}
			valueSort, cause := source.ast.GetSort(column.constant)
			if cause != nil || singletonSorts[0] != valueSort {
				return nil, errors.Join(cause, ErrTableRelation)
			}
			continue
		}
		if uint64(column.source) >= uint64(len(sorts)) {
			return nil, ast.ErrASTDomain
		}
		referenced[column.source] = true
	}
	current, err := source.Clone()
	if err != nil {
		return nil, err
	}
	owner := &externalRuleExecutionOwner{runtime: source.ast.NumeralService().RuntimeOwner()}
	current = owner.adopt(current)
	defer func() {
		resultErr = finishExternalRuleExecution(owner, resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	closeOwned := func(relation *ExternalRelation) error { return owner.closeOne(relation) }
	removed := make([]uint32, 0, len(sorts))
	compact := make(map[uint32]uint32)
	for index := range sorts {
		if referenced[index] {
			compact[uint32(index)] = uint32(len(compact))
		} else {
			removed = append(removed, uint32(index))
		}
	}
	if len(removed) != 0 {
		next, cause := current.Project(removed)
		next = owner.adopt(next)
		cause = errors.Join(cause, closeOwned(current))
		if cause != nil {
			cause = errors.Join(cause, closeOwned(next))
			return nil, cause
		}
		current = next
	}
	assigned := make([]uint32, len(columns))
	used := make(map[uint32]bool)
	for destination, column := range columns {
		currentSorts, cause := current.Sorts()
		if cause != nil {
			return nil, cause
		}
		if !column.constant.IsNull() {
			singleton := constants[column.constant]
			next, joinErr := JoinExternalRelations(current, singleton, nil, nil)
			next = owner.adopt(next)
			joinErr = errors.Join(joinErr, closeOwned(current))
			if joinErr != nil {
				joinErr = errors.Join(joinErr, closeOwned(next))
				return nil, joinErr
			}
			assigned[destination] = uint32(len(currentSorts))
			current = next
			continue
		}
		columnIndex := compact[column.source]
		if !used[column.source] {
			used[column.source] = true
			assigned[destination] = columnIndex
			continue
		}
		removeOther := make([]uint32, 0, len(currentSorts)-1)
		for index := range currentSorts {
			if uint32(index) != columnIndex {
				removeOther = append(removeOther, uint32(index))
			}
		}
		var singleton *ExternalRelation
		if len(removeOther) == 0 {
			singleton, cause = current.Clone()
		} else {
			singleton, cause = current.Project(removeOther)
		}
		if cause != nil {
			return nil, cause
		}
		singleton = owner.adopt(singleton)
		next, joinErr := JoinExternalRelations(current, singleton, []uint32{columnIndex}, []uint32{0})
		next = owner.adopt(next)
		joinErr = errors.Join(joinErr, closeOwned(singleton), closeOwned(current))
		if joinErr != nil {
			joinErr = errors.Join(joinErr, closeOwned(next))
			return nil, joinErr
		}
		assigned[destination] = uint32(len(currentSorts))
		current = next
	}
	visited := make([]bool, len(assigned))
	for start := range assigned {
		if visited[start] || assigned[start] == uint32(start) {
			visited[start] = true
			continue
		}
		cycle := make([]uint32, 0, len(assigned))
		position := uint32(start)
		for !visited[position] {
			visited[position] = true
			cycle = append(cycle, position)
			position = assigned[position]
		}
		if position != uint32(start) {
			return nil, ErrCompilerUnsupported
		}
		next, renameErr := current.Rename(cycle)
		next = owner.adopt(next)
		renameErr = errors.Join(renameErr, closeOwned(current))
		if renameErr != nil {
			renameErr = errors.Join(renameErr, closeOwned(next))
			return nil, renameErr
		}
		current = next
	}
	result = owner.release(current)
	return result, nil
}
