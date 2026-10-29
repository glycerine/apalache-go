package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1513-1523,1555-1566;
// src/ast/rewriter/bv_rewriter.h:87-88,177.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteZeroExtend(n uint32, arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	if n == 0 {
		return RewriteDone, p.manager.SetRef(out, arg)
	}
	p.extensionPair[0], err = p.manager.BitVectorNumeralUnsignedWithInvocation(0, n, &p.scalarNumeral)
	if err != nil {
		return
	}
	p.extensionPair[1] = arg
	node, e := p.manager.MkBitVectorConcat(p.extensionPair[:])
	if e != nil {
		return st, e
	}
	st = Rewrite1
	if err = p.manager.SetRef(out, node); err != nil {
		return
	}
	p.extensionPair = [2]ast.ASTHandle{}
	return
}

func (p *BVRewriter) RewriteRepeat(n uint32, arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	if n == 1 {
		return RewriteDone, p.manager.SetRef(out, arg)
	}
	for i := uint32(0); i < n; i++ {
		if p.repeatArgs = append(p.repeatArgs, arg); err != nil {
			return
		}
	}
	node, e := p.manager.MkBitVectorConcat(p.repeatArgs)
	if e != nil {
		return st, e
	}
	st = Rewrite1
	if err = p.manager.SetRef(out, node); err != nil {
		return
	}
	clear(p.repeatArgs)
	p.repeatArgs = nil
	return
}
