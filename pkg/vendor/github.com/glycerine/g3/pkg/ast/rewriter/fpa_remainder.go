package rewriter

// Source: src/ast/rewriter/fpa_rewriter.cpp:340-353.

import "github.com/glycerine/g3/pkg/ast"

func (r *FPARewriter) fpaRemainder(left, right ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	value, found, err := r.manager.RewriteFPARemainderNumerals(left, right)
	if err != nil || !found {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, value)
}
