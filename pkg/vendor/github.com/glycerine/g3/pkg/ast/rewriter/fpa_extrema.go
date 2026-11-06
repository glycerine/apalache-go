package rewriter

// Source: src/ast/rewriter/fpa_rewriter.cpp:369-415.

import "github.com/glycerine/g3/pkg/ast"

func (r *FPARewriter) fpaExtremum(left, right ast.ASTHandle, kind int32, out ast.ASTRef) (RewriteStatus, error) {
	nan, err := r.fpaIsNaN(left)
	if err != nil {
		return RewriteFailed, err
	}
	if nan {
		return RewriteDone, r.manager.SetRef(out, right)
	}
	nan, err = r.fpaIsNaN(right)
	if err != nil {
		return RewriteFailed, err
	}
	if nan {
		return RewriteDone, r.manager.SetRef(out, left)
	}
	value, found, err := r.manager.RewriteFPAExtremumNumerals(left, right, kind)
	if err != nil || !found {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, value)
}
