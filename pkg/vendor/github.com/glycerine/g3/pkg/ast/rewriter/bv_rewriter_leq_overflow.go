package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:255-346;
// src/ast/rewriter/bv_rewriter.h:80-81.

import (
	"github.com/glycerine/g3/pkg/ast"
	"reflect"
)

type bvLEOverflowInvocation struct {
	common     ast.ASTRef
	numerals   *ast.BitVectorOverflowComparisonNumerals
	difference bool
	header     uint64
}

func (v *bvLEOverflowInvocation) close(m *ast.ASTManager) error {
	if v.numerals != nil {
		if e := v.numerals.Close(); e != nil {
			return e
		}
		v.numerals = nil
	}
	if v.common != (ast.ASTRef{}) {
		if e := m.DelRef(&v.common); e != nil {
			return e
		}
	}
	m.Budget().Release(v.header)
	v.header = 0
	return nil
}
func (p *BVRewriter) RewriteLEOverflow(signed bool, a, b ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.rewriteLEOverflow(signed, a, b, out)
}
func (p *BVRewriter) rewriteLEOverflow(signed bool, a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if signed {
		return RewriteFailed, nil
	}
	m := p.manager
	bytes := uint64(reflect.TypeOf(bvLEOverflowInvocation{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return RewriteFailed, e
	}
	v := &bvLEOverflowInvocation{header: bytes}
	p.leOverflow = v
	var e error
	// Original default common ref precedes both default numerical values.
	v.common, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return RewriteFailed, e
	}
	v.numerals, e = ast.NewBitVectorOverflowComparisonNumerals(m)
	if e != nil {
		return RewriteFailed, e
	}
	node, st, e := p.leOverflowBody(v, a, b)
	if e != nil {
		return st, e
	}
	if st != RewriteFailed {
		if e = m.SetRef(out, node); e != nil {
			return st, e
		}
	}
	// The less-offset branch's subtraction is an expression temporary.
	if v.difference {
		if e = v.numerals.End(2); e != nil {
			return st, e
		}
	}
	if e = v.numerals.SourceEnd(); e != nil {
		return st, e
	}
	if e = m.DelRef(&v.common); e != nil {
		return st, e
	}
	if e = v.close(m); e != nil {
		return st, e
	}
	p.leOverflow = nil
	return st, nil
}
func (p *BVRewriter) areEqualUpToNumeral(v *bvLEOverflowInvocation, a, b ast.ASTHandle) (bool, error) {
	m := p.manager
	n := v.numerals
	aadd, e := m.IsBitVectorOperation(a, 4)
	if e != nil {
		return false, e
	}
	badd, e := m.IsBitVectorOperation(b, 4)
	if e != nil {
		return false, e
	}
	has := func(add bool, h ast.ASTHandle) (bool, error) {
		if !add {
			return false, nil
		}
		args, e := m.AppArgumentView(h)
		if e != nil || args.Count() == 0 {
			return false, e
		}
		first, e := args.At(0)
		if e != nil {
			return false, e
		}
		return m.IsBitVectorNumeral(first)
	}
	anum, e := has(aadd, a)
	if e != nil {
		return false, e
	}
	bnum, e := has(badd, b)
	if e != nil {
		return false, e
	}
	if e = n.Zero(0); e != nil {
		return false, e
	}
	if e = n.Zero(1); e != nil {
		return false, e
	}
	if !aadd && !badd {
		if a != b {
			return false, nil
		}
		return true, m.SetRef(v.common, a)
	}
	// Preserve the source's mixed-case operands and short-circuit order. Its
	// has_num test is false on the non-ADD side, so these branches reject after
	// reading that side's arity. Non-application to_app domains remain guarded.
	if !aadd && badd {
		aa, e := m.AppArgumentView(a)
		if e != nil {
			return false, e
		}
		if aa.Count() != 2 || !anum {
			return false, nil
		}
		first, e := aa.At(0)
		if e != nil || first != b {
			return false, e
		}
		return true, m.SetRef(v.common, b)
	}
	if aadd && !badd {
		bb, e := m.AppArgumentView(b)
		if e != nil {
			return false, e
		}
		if bb.Count() != 2 || !bnum {
			return false, nil
		}
		first, e := bb.At(0)
		if e != nil || first != a {
			return false, e
		}
		return true, m.SetRef(v.common, a)
	}
	aa, e := m.AppArgumentView(a)
	if e != nil {
		return false, e
	}
	bb, e := m.AppArgumentView(b)
	if e != nil {
		return false, e
	}
	numa, numb := aa.Count(), bb.Count()
	if numa == 0 || numb == 0 {
		return false, nil
	}
	var ai, bi uint32
	if anum {
		ai = 1
	}
	if bnum {
		bi = 1
	}
	if numa-ai != numb-bi {
		return false, nil
	}
	for i, j := ai, bi; i < numa; i, j = i+1, j+1 {
		x, e := aa.At(i)
		if e != nil {
			return false, e
		}
		y, e := bb.At(j)
		if e != nil {
			return false, e
		}
		if x != y {
			return false, nil
		}
	}
	// Source repeats both static-zero assignments after the commonality scan.
	if e = n.Zero(0); e != nil {
		return false, e
	}
	if e = n.Zero(1); e != nil {
		return false, e
	}
	width, e := m.BitVectorSize(a)
	if e != nil {
		return false, e
	}
	as, bs := width, width
	if anum {
		first, e := aa.At(0)
		if e != nil {
			return false, e
		}
		_, e = n.Read(0, first, &as)
		if e != nil {
			return false, e
		}
	}
	if bnum {
		first, e := bb.At(0)
		if e != nil {
			return false, e
		}
		_, e = n.Read(1, first, &bs)
		if e != nil {
			return false, e
		}
	}
	if ast.ASTSourceDebug {
		w, e := m.BitVectorSize(a)
		if e != nil {
			return false, e
		}
		if as != w {
			return false, ast.ErrASTDomain
		}
		w, e = m.BitVectorSize(a)
		if e != nil {
			return false, e
		}
		if bs != w {
			return false, ast.ErrASTDomain
		}
	}
	common := a
	if anum && numa > 2 {
		suffix, e := aa.Range(1, numa-1)
		if e != nil {
			return false, e
		}
		common, e = m.MkBitVectorAddArguments(suffix)
		if e != nil {
			return false, e
		}
	} else if anum {
		common, e = aa.At(1)
		if e != nil {
			return false, e
		}
	}
	return true, m.SetRef(v.common, common)
}
func (p *BVRewriter) leOverflowBody(v *bvLEOverflowInvocation, a, b ast.ASTHandle) (ast.ASTHandle, RewriteStatus, error) {
	m := p.manager
	n := v.numerals
	failed := ast.ASTHandle{}
	matched, e := p.areEqualUpToNumeral(v, a, b)
	if e != nil || !matched {
		return failed, RewriteFailed, e
	}
	if ast.ASTSourceDebug {
		yes, e := n.IsNonnegative(0)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if !yes {
			return failed, RewriteFailed, ast.ErrASTDomain
		}
		yes, e = n.IsNonnegative(1)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if !yes {
			return failed, RewriteFailed, ast.ErrASTDomain
		}
	}
	width, e := m.BitVectorSize(a)
	if e != nil {
		return failed, RewriteFailed, e
	}
	equal, e := n.Equal(0, 1)
	if e != nil {
		return failed, RewriteFailed, e
	}
	if equal {
		node, e := m.MkTrue()
		return node, RewriteDone, e
	}
	less, e := n.Less(0, 1)
	if e != nil {
		return failed, RewriteFailed, e
	}
	if less {
		if e = n.Difference(); e != nil {
			return failed, RewriteFailed, e
		}
		v.difference = true
		c, e := n.Constant(2, width)
		if e != nil {
			return failed, RewriteFailed, e
		}
		node, e := m.MkBitVectorULE(c, b)
		return node, Rewrite2, e
	}
	if ast.ASTSourceDebug {
		greater, e := n.Less(1, 0)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if !greater {
			return failed, RewriteFailed, ast.ErrASTDomain
		}
		zero, e := n.IsZero(0)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if zero {
			return failed, RewriteFailed, ast.ErrASTDomain
		}
	}
	if e = n.Bounds(width); e != nil {
		return failed, RewriteFailed, e
	}
	equal, e = n.Equal(2, 3)
	if e != nil {
		return failed, RewriteFailed, e
	}
	common, e := m.RefGet(v.common)
	if e != nil {
		return failed, RewriteFailed, e
	}
	if equal {
		c, e := n.Constant(2, width)
		if e != nil {
			return failed, RewriteFailed, e
		}
		node, e := m.MkEq(common, c)
		return node, Rewrite2, e
	}
	zero, e := n.IsZero(1)
	if e != nil {
		return failed, RewriteFailed, e
	}
	if zero {
		c, e := n.Constant(2, width)
		if e != nil {
			return failed, RewriteFailed, e
		}
		node, e := m.MkBitVectorULE(c, common)
		return node, Rewrite2, e
	}
	if ast.ASTSourceDebug {
		yes, e := n.IsPositive(2)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if !yes {
			return failed, RewriteFailed, ast.ErrASTDomain
		}
	}
	lower, e := n.Constant(2, width)
	if e != nil {
		return failed, RewriteFailed, e
	}
	left, e := m.MkBitVectorULE(lower, common)
	if e != nil {
		return failed, RewriteFailed, e
	}
	upper, e := n.Constant(3, width)
	if e != nil {
		return failed, RewriteFailed, e
	}
	right, e := m.MkBitVectorULE(common, upper)
	if e != nil {
		return failed, RewriteFailed, e
	}
	node, e := m.MkAndArgs([]ast.ASTHandle{left, right})
	return node, Rewrite2, e
}
