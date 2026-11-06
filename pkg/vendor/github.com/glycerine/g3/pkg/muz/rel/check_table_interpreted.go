package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpreted applies the same condition to the checker and primary
// tables in the source order, then verifies that the two results still match.
func (t *CheckTable) FilterInterpreted(manager *RelationManager, relationSorts []ast.ASTHandle, rewriter *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	if err := t.open(); err != nil {
		return err
	}
	checker := &TableRelation{manager: manager, sorts: append([]ast.ASTHandle(nil), relationSorts...), table: t.checker}
	if err := checker.FilterInterpreted(rewriter, conditions, columns, sorts); err != nil {
		return err
	}
	primary := &TableRelation{manager: manager, sorts: append([]ast.ASTHandle(nil), relationSorts...), table: t.primary}
	if err := primary.FilterInterpreted(rewriter, conditions, columns, sorts); err != nil {
		return err
	}
	return t.Validate()
}

// FilterInterpretedAndProject returns an independently owned checked result,
// applying the source checker's transform before the primary transform.
func (t *CheckTable) FilterInterpretedAndProject(manager *RelationManager, relationSorts []ast.ASTHandle, rewriter *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle, removed []int) (_ *CheckTable, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	checkerRelation := &TableRelation{manager: manager, sorts: append([]ast.ASTHandle(nil), relationSorts...), table: t.checker}
	checkerResult, err := checkerRelation.FilterInterpretedAndProject(rewriter, conditions, columns, sorts, removed)
	if err != nil {
		return nil, err
	}
	checkerTable := checkerResult.Table()
	checkerResult.Close()
	primaryRelation := &TableRelation{manager: manager, sorts: append([]ast.ASTHandle(nil), relationSorts...), table: t.primary}
	primaryResult, err := primaryRelation.FilterInterpretedAndProject(rewriter, conditions, columns, sorts, removed)
	if err != nil {
		return nil, errors.Join(err, checkerTable.Close())
	}
	primaryTable := primaryResult.Table()
	primaryResult.Close()
	result, err := AdoptCheckTable(primaryTable, checkerTable)
	if err != nil {
		return nil, errors.Join(err, primaryTable.Close(), checkerTable.Close())
	}
	return result, nil
}
