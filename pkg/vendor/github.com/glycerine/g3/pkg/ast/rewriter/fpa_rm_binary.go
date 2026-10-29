package rewriter

// Source: src/ast/rewriter/fpa_rewriter.cpp:256-307.

import "github.com/glycerine/g3/pkg/ast"

func (r *FPARewriter) fpaRMBinary(rm, left, right ast.ASTHandle, kind int32, out ast.ASTRef) (RewriteStatus, error) {
	answer, found, err := r.manager.RewriteFPARMBinaryNumerals(rm, left, right, kind)
	if err != nil || !found {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, answer)
}

func (r *FPARewriter) fpaSub(rm, left, right ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	negated, err := r.manager.MkFamilyApp(r.family, ast.FPAOpNeg, nil, []ast.ASTHandle{right})
	if err != nil {
		return RewriteFailed, err
	}
	addition, err := r.manager.MkFamilyApp(r.family, ast.FPAOpAdd, nil, []ast.ASTHandle{rm, left, negated})
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, r.manager.SetRef(out, addition)
}
