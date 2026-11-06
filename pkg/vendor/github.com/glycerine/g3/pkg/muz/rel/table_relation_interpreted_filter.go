package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpreted applies the concrete finite-table interpreted filter. It
// determines every removal before mutating the relation, so evaluation errors
// leave the source table unchanged.
func (r *TableRelation) FilterInterpreted(rewriter *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	if err := r.validateInterpretedFilter(rewriter, conditions, columns, sorts); err != nil {
		return err
	}
	removed := make([]string, 0)
	for key, fact := range r.table.rows.All() {
		accepted, err := r.acceptsInterpretedFact(rewriter, conditions, columns, sorts, fact)
		if err != nil {
			return err
		}
		if !accepted {
			removed = append(removed, key)
		}
	}
	for _, key := range removed {
		r.table.rows.Del(key)
	}
	return nil
}
