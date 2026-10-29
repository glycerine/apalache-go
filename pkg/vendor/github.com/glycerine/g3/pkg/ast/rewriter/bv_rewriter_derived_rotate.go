package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2000-2018;
// src/ast/rewriter/bv_rewriter.h:98-99.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteRotateLeft(n uint32, arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.rotateLeft(n, arg, out)
}
func (p *BVRewriter) RewriteRotateRight(n uint32, arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.rotateRight(n, arg, out)
}

func (p *BVRewriter) rotateRight(n uint32, arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	size, e := p.manager.BitVectorSize(arg)
	if e != nil {
		return RewriteFailed, e
	}
	// The original DEBUG assertion precedes modulo. Invalid zero-width sorts
	// retain the existing guarded-domain adaptation in release builds too.
	if size == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	n %= size
	return p.rotateLeft(size-n, arg, out)
}

func (p *BVRewriter) rotateLeft(n uint32, arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	m := p.manager
	size, e := m.BitVectorSize(arg)
	if e != nil {
		return RewriteFailed, e
	}
	if size == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	n %= size
	if n == 0 || size == 1 {
		return RewriteDone, m.SetRef(out, arg)
	}
	p.rotationArgs[0], e = p.extract.Apply(size-n-1, 0, arg)
	if e != nil {
		return RewriteFailed, e
	}
	p.rotationArgs[1], e = p.extract.Apply(size-1, size-n, arg)
	if e != nil {
		return RewriteFailed, e
	}
	result, e := m.MkBitVectorConcat(p.rotationArgs[:])
	if e != nil {
		return RewriteFailed, e
	}
	if e = m.SetRef(out, result); e != nil {
		return Rewrite2, e
	}
	p.rotationArgs = [2]ast.ASTHandle{}
	return Rewrite2, nil
}
