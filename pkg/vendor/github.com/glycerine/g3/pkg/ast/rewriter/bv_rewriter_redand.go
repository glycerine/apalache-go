package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2048-2056.

import "github.com/glycerine/g3/pkg/ast"

func RewriteBitVectorReductionAND(m *ast.ASTManager, arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if m == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	matched, e := m.ReduceBitVectorAND(arg, out)
	if e != nil || !matched {
		return RewriteFailed, e
	}
	return RewriteDone, nil
}
