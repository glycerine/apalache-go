package rewriter

// Source: src/ast/rewriter/fpa_rewriter.cpp:417-431.

import "github.com/glycerine/g3/pkg/ast"

func (r *FPARewriter) fpaFMA(rm, x, y, z ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	answer, found, err := r.manager.RewriteFPAFMANumerals(rm, x, y, z)
	if err != nil || !found {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, answer)
}
