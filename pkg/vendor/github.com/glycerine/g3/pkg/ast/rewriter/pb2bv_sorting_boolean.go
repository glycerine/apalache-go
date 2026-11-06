package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp mixed-radix mk_and/mk_or.

import "github.com/glycerine/g3/pkg/ast"

func (r *PB2BVRewriter) sortingAnd(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	if r.manager.IsTrue(left) {
		return right, nil
	}
	if r.manager.IsTrue(right) || r.manager.IsFalse(left) {
		return left, nil
	}
	if r.manager.IsFalse(right) {
		return right, nil
	}
	return r.manager.MkAnd(left, right)
}

func (r *PB2BVRewriter) sortingOr(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	if r.manager.IsTrue(left) || r.manager.IsFalse(right) {
		return left, nil
	}
	if r.manager.IsTrue(right) || r.manager.IsFalse(left) {
		return right, nil
	}
	return r.manager.MkOr(left, right)
}
