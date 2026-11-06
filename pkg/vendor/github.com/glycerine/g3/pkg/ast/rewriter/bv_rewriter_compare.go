package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2040-2046,2058-2074.

import "github.com/glycerine/g3/pkg/ast"

func RewriteBitVectorReductionOR(m *ast.ASTManager, arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if m == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	numeral, e := m.IsBitVectorNumeral(arg)
	if e != nil || !numeral {
		return RewriteFailed, e
	}
	zero, e := m.IsBitVectorZero(arg)
	if e != nil {
		return RewriteFailed, e
	}
	value := uint32(1)
	if zero {
		value = 0
	}
	h, e := m.BitVectorNumeralUnsigned(value, 1)
	if e != nil {
		return RewriteFailed, e
	}
	return RewriteDone, m.SetRef(out, h)
}

func RewriteBitVectorCompare(m *ast.ASTManager, left, right ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if m == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	if left == right {
		h, e := m.BitVectorNumeralUnsigned(1, 1)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, m.SetRef(out, h)
	}
	first, e := m.IsBitVectorNumeral(left)
	if e != nil {
		return RewriteFailed, e
	}
	if first {
		second, e := m.IsBitVectorNumeral(right)
		if e != nil {
			return RewriteFailed, e
		}
		if second {
			h, e := m.BitVectorNumeralUnsigned(0, 1)
			if e != nil {
				return RewriteFailed, e
			}
			return RewriteDone, m.SetRef(out, h)
		}
	}
	eq, e := m.MkEq(left, right)
	if e != nil {
		return RewriteFailed, e
	}
	one, e := m.BitVectorNumeralUnsigned(1, 1)
	if e != nil {
		return RewriteFailed, e
	}
	zero, e := m.BitVectorNumeralUnsigned(0, 1)
	if e != nil {
		return RewriteFailed, e
	}
	h, e := m.MkIte(eq, one, zero)
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite2, m.SetRef(out, h)
}
