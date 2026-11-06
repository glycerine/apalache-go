package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// filterSparseInterpretedBound handles compiled guards whose variable indices
// do not necessarily equal relation columns. It computes all removals before
// mutating packed storage.
func filterSparseInterpretedBound(table *SparseTable, manager *RelationManager, rewriter *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	if table == nil || table.open() != nil || manager == nil || manager.open() != nil || rewriter == nil || rewriter.Manager() != manager.manager || len(conditions) == 0 {
		return ErrTable
	}
	for variable, sort := range sorts {
		column, found := columns[variable]
		if !found || column < 0 || column >= table.signature.Len() {
			return ErrTable
		}
		domain, err := manager.sortDomain(sort)
		if err != nil || domain != table.signature.domains[column] {
			return ErrTable
		}
	}
	facts, err := table.Facts()
	if err != nil {
		return err
	}
	relation := &TableRelation{manager: manager}
	removed := make([]Fact, 0)
	for _, fact := range facts {
		accepted, evalErr := relation.acceptsInterpretedFact(rewriter, conditions, columns, sorts, fact)
		if evalErr != nil {
			return evalErr
		}
		if !accepted {
			removed = append(removed, fact)
		}
	}
	_, err = table.RemoveFacts(removed)
	return err
}
