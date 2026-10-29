package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1513-1523,1555-1566;
// src/ast/rewriter/bv_rewriter.h:177.

import "github.com/glycerine/g3/pkg/ast"

func RewriteBitVectorZeroExtend(m *ast.ASTManager, n uint32, arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if m == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	if n == 0 {
		return RewriteDone, m.SetRef(out, arg)
	}
	zero, e := m.BitVectorNumeralUnsigned(0, n)
	if e != nil {
		return RewriteFailed, e
	}
	h, e := m.MkBitVectorConcatPair(zero, arg)
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite1, m.SetRef(out, h)
}

func RewriteBitVectorRepeat(m *ast.ASTManager, n uint32, arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if m == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	if n == 1 {
		return RewriteDone, m.SetRef(out, arg)
	}
	var args []ast.ASTHandle
	for i := uint32(0); i < n; i++ {
		args = append(args, arg)
	}
	h, e := m.MkBitVectorConcat(args)
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite1, m.SetRef(out, h)
}
