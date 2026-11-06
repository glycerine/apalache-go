package rewriter

// Source: src/ast/rewriter/fpa_rewriter.cpp:433-466.

import "github.com/glycerine/g3/pkg/ast"

func (r *FPARewriter) fpaRMUnary(rm, value ast.ASTHandle, kind int32, out ast.ASTRef) (RewriteStatus, error) {
	answer, found, err := r.manager.RewriteFPARMUnaryNumeral(rm, value, kind)
	if err != nil || !found {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, answer)
}
