package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpretedAndProject mirrors the source fused transformer while
// leaving its input relation unchanged.
func (r *TableRelation) FilterInterpretedAndProject(rewriter *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle, removed []int) (*TableRelation, error) {
	clone, err := r.Clone()
	if err != nil {
		return nil, err
	}
	if err = clone.FilterInterpreted(rewriter, conditions, columns, sorts); err != nil {
		return nil, errors.Join(err, clone.Close())
	}
	projected, err := clone.Project(removed)
	closeErr := clone.Close()
	if err != nil {
		return nil, errors.Join(err, closeErr)
	}
	return projected, closeErr
}
