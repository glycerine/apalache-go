package rewriter

// Source: src/ast/rewriter/poly_rewriter_def.h:855-858.

import (
	"github.com/glycerine/g3/pkg/ast"
	"sort"
)

// Go sorting uses the project comparator; library callback/swap order is not
// part of the project algorithm. Preserve its first domain error.
func bvPolynomialSortMonomials(m *ast.ASTManager, fid ast.FamilyID, order bool, v []ast.ASTHandle) error {
	var cause error
	sort.Slice(v, func(i, j int) bool {
		if cause != nil {
			return false
		}
		var less bool
		less, cause = bvMonomialLess(m, fid, order, v[i], v[j])
		return less
	})
	return cause
}
