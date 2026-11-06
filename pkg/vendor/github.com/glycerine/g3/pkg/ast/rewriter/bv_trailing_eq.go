package rewriter

// Source: src/ast/rewriter/bv_trailing.cpp:51-85,392-394.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVTrailing) EqRemoveTrailing(left, right ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, body error) {
	if e := p.enter(); e != nil {
		return RewriteFailed, e
	}
	defer func() { p.leave(body) }()
	m := p.manager
	if e := p.traceEq(left, right); e != nil {
		return RewriteFailed, e
	}
	if ast.ASTSourceDebug {
		for _, arg := range []ast.ASTHandle{left, right} {
			yes, e := m.IsBitVector(arg)
			if e != nil {
				return RewriteFailed, e
			}
			if !yes {
				return RewriteFailed, ast.ErrASTDomain
			}
		}
		w1, e := m.BitVectorSize(left)
		if e != nil {
			return RewriteFailed, e
		}
		w2, e := m.BitVectorSize(right)
		if e != nil {
			return RewriteFailed, e
		}
		if w1 != w2 {
			return RewriteFailed, ast.ErrASTDomain
		}
	}
	p.eqCall.child = nil
	min1, max1, e := p.countTracked(left, bvTrailingDepth, &p.eqCall)
	if e != nil {
		return RewriteFailed, e
	}
	min2, max2, e := p.countTracked(right, bvTrailingDepth, &p.eqCall)
	if e != nil {
		return RewriteFailed, e
	}
	if min1 > max2 || min2 > max1 {
		h, e := m.MkFalse()
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, m.SetRef(out, h)
	}
	min := min1
	if min2 < min {
		min = min2
	}
	if min == 0 {
		h, e := m.MkEq(left, right)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteFailed, m.SetRef(out, h)
	}
	width, e := m.BitVectorSize(left)
	if e != nil {
		return RewriteFailed, e
	}
	if min == width {
		h, e := m.MkTrue()
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, m.SetRef(out, h)
	}
	f, e := p.beginFrame(bvTrailingEqFrame, &p.eqCall)
	if e != nil {
		return RewriteFailed, e
	}
	defer f.finish(&body)
	if e = f.newRef(0); e != nil {
		return RewriteFailed, e
	}
	if e = f.newRef(1); e != nil {
		return RewriteFailed, e
	}
	for i, arg := range []ast.ASTHandle{left, right} {
		removed, e := p.remove(arg, min, f.refs[i], bvTrailingDepth, &f.call)
		if e != nil {
			return RewriteFailed, e
		}
		if removed != min {
			return RewriteFailed, ast.ErrASTDomain
		}
	}
	a, e := m.RefGet(f.refs[0])
	if e != nil {
		return RewriteFailed, e
	}
	b, e := m.RefGet(f.refs[1])
	if e != nil {
		return RewriteFailed, e
	}
	equal, e := m.AreEqual(a, b)
	if e != nil {
		return RewriteFailed, e
	}
	var result ast.ASTHandle
	if equal {
		result, e = m.MkTrue()
		status = RewriteDone
	} else {
		result, e = m.MkEq(a, b)
		status = Rewrite2
	}
	if e != nil {
		return RewriteFailed, e
	}
	return status, m.SetRef(out, result)
}
