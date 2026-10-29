package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1971-1998.

import "github.com/glycerine/g3/pkg/ast"

// These core reducers borrow the manager and operand slice. They expose the
// original algorithms independently of the remaining BV rewriter traversal.
func RewriteBitVectorNAND(m *ast.ASTManager, args []ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if m == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	var newArgs []ast.ASTHandle
	for _, arg := range args {
		neg, e := m.MkBitVectorNot(arg)
		if e != nil {
			return RewriteFailed, e
		}
		newArgs = append(newArgs, neg)
	}
	h, e := m.MkBitVectorOr(newArgs)
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite2, m.SetRef(out, h)
}

func RewriteBitVectorNOR(m *ast.ASTManager, args []ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if m == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	inner, e := m.MkBitVectorOr(args)
	if e != nil {
		return RewriteFailed, e
	}
	h, e := m.MkBitVectorNot(inner)
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite2, m.SetRef(out, h)
}

func RewriteBitVectorXNOR(m *ast.ASTManager, args []ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if m == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	inner, e := m.MkBitVectorXor(args)
	if e != nil {
		return RewriteFailed, e
	}
	h, e := m.MkBitVectorNot(inner)
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite2, m.SetRef(out, h)
}

// RewriteBitVectorAND preserves the original De Morgan expansion and depth.
func RewriteBitVectorAND(m *ast.ASTManager, args []ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if m == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	var newArgs []ast.ASTHandle
	for _, arg := range args {
		neg, e := m.MkBitVectorNot(arg)
		if e != nil {
			return RewriteFailed, e
		}
		newArgs = append(newArgs, neg)
	}
	inner, e := m.MkBitVectorOr(newArgs)
	if e != nil {
		return RewriteFailed, e
	}
	h, e := m.MkBitVectorNot(inner)
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite3, m.SetRef(out, h)
}
