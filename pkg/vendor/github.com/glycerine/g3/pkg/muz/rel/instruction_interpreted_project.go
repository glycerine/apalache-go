package rel

// Source: src/muz/rel/dl_instruction.cpp
// instr_filter_interpreted_and_project.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

type interpretedFilterProjectInstruction struct {
	*interpretedFilterInstruction
	result        uint32
	removed       []int
	relationSorts []ast.ASTHandle
}

// NewFilterInterpretedAndProjectInstruction constructs the non-mutating
// interpreted filter followed by projection used by the finite compiler.
func NewFilterInterpretedAndProjectInstruction(manager *ast.ASTManager, relations *RelationManager, rewriter *astrewriter.TheoryRewriter, source uint32, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle, relationSorts []ast.ASTHandle, removed []int, result uint32) (Instruction, error) {
	if source == VoidRegister || result == VoidRegister || len(removed) == 0 {
		return nil, ErrInstruction
	}
	previous := -1
	for _, column := range removed {
		if column <= previous {
			return nil, ErrInstruction
		}
		previous = column
	}
	filter, err := newInterpretedFilterInstruction(manager, relations, rewriter, source, conditions, columns, sorts)
	if err != nil {
		return nil, err
	}
	return &interpretedFilterProjectInstruction{
		interpretedFilterInstruction: filter,
		result:                       result,
		removed:                      append([]int(nil), removed...),
		relationSorts:                append([]ast.ASTHandle(nil), relationSorts...),
	}, nil
}

func (i *interpretedFilterProjectInstruction) projectHash(table *HashTable, conditions []ast.ASTHandle) (_ FiniteTable, resultErr error) {
	relation := &TableRelation{
		manager: i.relations,
		sorts:   append([]ast.ASTHandle(nil), i.relationSorts...),
		table:   table,
	}
	projected, err := relation.FilterInterpretedAndProject(i.rewriter, conditions, i.variableColumns, i.variableSorts, i.removed)
	if err != nil {
		return nil, err
	}
	result := projected.Table()
	if result == nil {
		return nil, errors.Join(ErrInstruction, projected.Close())
	}
	if err = projected.Close(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

func (i *interpretedFilterProjectInstruction) projectSparse(table *SparseTable, conditions []ast.ASTHandle) (_ FiniteTable, resultErr error) {
	clone, err := table.Clone()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, clone.Close()) }()
	if err = filterSparseInterpretedBound(clone, i.relations, i.rewriter, conditions, i.variableColumns, i.variableSorts); err != nil {
		return nil, err
	}
	return clone.Project(i.removed)
}

func (i *interpretedFilterProjectInstruction) projectLazySparse(table *LazySparseTable, conditions []ast.ASTHandle) (_ FiniteTable, resultErr error) {
	clone, err := table.Clone()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, clone.Close()) }()
	if err = clone.filterInterpretedCompiled(i.relations, i.rewriter, conditions, i.variableColumns, i.variableSorts); err != nil {
		return nil, err
	}
	return clone.Project(i.removed)
}

func (i *interpretedFilterProjectInstruction) project(table FiniteTable, conditions []ast.ASTHandle) (FiniteTable, error) {
	switch concrete := table.(type) {
	case *HashTable:
		return i.projectHash(concrete, conditions)
	case *SparseTable:
		return i.projectSparse(concrete, conditions)
	case *LazySparseTable:
		return i.projectLazySparse(concrete, conditions)
	case *FiniteCheckedPair:
		return concrete.FilterInterpretedAndProject(i.relations, i.relationSorts, i.rewriter, conditions, i.variableColumns, i.variableSorts, i.removed)
	default:
		return nil, ErrInstruction
	}
}

func (i *interpretedFilterProjectInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	if err := context.open(); err != nil {
		return false, err
	}
	source, err := context.FiniteRegister(i.register)
	if err != nil {
		return false, err
	}
	if source == nil {
		return true, context.MakeEmpty(i.result)
	}
	context.stats.FilterInterpretedProject++
	conditions, err := i.conditionHandles()
	if err != nil {
		return false, err
	}
	result, err := i.project(source, conditions)
	if err != nil {
		return false, err
	}
	if _, lazy := result.(*LazySparseTable); lazy {
		err = context.SetFiniteRegister(i.result, result)
	} else {
		err = setNonemptyFiniteRegister(context, i.result, result)
	}
	if err != nil {
		return false, errors.Join(err, result.Close())
	}
	return true, nil
}

func (i *interpretedFilterProjectInstruction) MakeAnnotations(context *ExecutionContext) error {
	if err := i.open(); err != nil {
		return err
	}
	return context.SetRegisterAnnotation(i.result, "filter_interpreted_and_project")
}

func (i *interpretedFilterProjectInstruction) DisplayHead(out io.Writer) error {
	if err := i.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	_, err := fmt.Fprintf(out, "filter_interpreted_and_project %d into %d deleting columns %s", i.register, i.result, columnsText(i.removed))
	return err
}
