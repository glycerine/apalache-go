package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2020-2038;
// src/ast/rewriter/bv_rewriter.h:100-101.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteExtendedRotateLeft(arg, amount ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteExtendedRotate(arg, amount, out, false)
}
func (p *BVRewriter) RewriteExtendedRotateRight(arg, amount ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteExtendedRotate(arg, amount, out, true)
}
func (p *BVRewriter) rewriteExtendedRotate(arg, amount ast.ASTHandle, out ast.ASTRef, right bool) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	n, e := ast.NewBitVectorExtendedRotateNumerals(p.manager)
	p.extendedRotate = n
	if e != nil {
		return st, e
	}
	var width uint32
	matched, e := n.Read(amount, &width)
	if e != nil {
		return st, e
	}
	if matched {
		shift, e := n.Shift(width)
		if e != nil {
			return st, e
		}
		if right {
			st, err = p.rotateRight(shift, arg, out)
		} else {
			st, err = p.rotateLeft(shift, arg, out)
		}
		if err != nil {
			return
		}
	}
	if err = n.SourceEnd(); err != nil {
		return
	}
	if err = n.Close(); err != nil {
		return
	}
	p.extendedRotate = nil
	return
}
