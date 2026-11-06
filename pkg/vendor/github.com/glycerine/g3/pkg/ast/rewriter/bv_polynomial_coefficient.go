package rewriter

// Source: src/ast/rewriter/poly_rewriter_def.h:418-426;
// src/ast/rewriter/bv_rewriter.h:36.

import (
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// bvCoefficientProduct copies into the caller's existing coefficient and only
// then publishes the borrowed power product. Unmatched inputs leave outputs
// and the caller's numerical invocation unread.
func bvCoefficientProduct(m *ast.ASTManager, t ast.ASTHandle, coefficient *unumeral.OwnedRational, product *ast.ASTHandle, c *unumeral.RationalServiceInvocation) (bool, error) {
	if m == nil {
		return false, ast.ErrASTHandle
	}
	mul, e := m.IsBitVectorOperation(t, 6)
	if e != nil || !mul {
		return false, e
	}
	count, e := m.GetNumArgs(t)
	if e != nil || count != 2 {
		return false, e
	}
	first, e := m.GetArg(t, 0)
	if e != nil {
		return false, e
	}
	var width uint32
	numeral, e := m.BitVectorNumeralValueWithInvocation(first, coefficient, &width, c)
	if e != nil || !numeral {
		return false, e
	}
	pp, e := m.GetArg(t, 1)
	if e != nil {
		return false, e
	}
	if product == nil {
		return false, ast.ErrASTHandle
	}
	*product = pp
	return true, nil
}
