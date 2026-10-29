package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:210-242;
// src/ast/rewriter/bv_rewriter.h:73-78.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteULE(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.RewriteLE(false, a, b, out)
}
func (p *BVRewriter) RewriteSLE(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.RewriteLE(true, a, b, out)
}
func (p *BVRewriter) RewriteUGE(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteGE(false, a, b, out)
}
func (p *BVRewriter) RewriteSGE(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteGE(true, a, b, out)
}
func (p *BVRewriter) rewriteGE(signed bool, a, b ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	st, err = p.rewriteLE(signed, b, a, out)
	if err != nil || st != RewriteFailed {
		return st, err
	}
	var node ast.ASTHandle
	if signed {
		node, err = p.manager.MkBitVectorSLE(b, a)
	} else {
		node, err = p.manager.MkBitVectorULE(b, a)
	}
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, p.manager.SetRef(out, node)
}
func (p *BVRewriter) RewriteULT(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteLT(false, a, b, out)
}
func (p *BVRewriter) RewriteSLT(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteLT(true, a, b, out)
}
func (p *BVRewriter) rewriteLT(signed bool, a, b ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	var le ast.ASTHandle
	if signed {
		le, err = p.manager.MkBitVectorSLE(b, a)
	} else {
		le, err = p.manager.MkBitVectorULE(b, a)
	}
	if err != nil {
		return RewriteFailed, err
	}
	node, e := p.manager.MkNot(le)
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite2, p.manager.SetRef(out, node)
}
