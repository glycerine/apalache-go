package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2040-2074;
// src/ast/rewriter/bv_rewriter.h:133-135.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) scalarBit(value uint32) (ast.ASTHandle, error) {
	return p.manager.BitVectorNumeralUnsignedWithInvocation(value, 1, &p.scalarNumeral)
}
func (p *BVRewriter) RewriteReductionOR(arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	numeral, e := m.IsBitVectorNumeral(arg)
	if e != nil || !numeral {
		return st, e
	}
	zero, e := m.IsBitVectorZero(arg)
	if e != nil {
		return st, e
	}
	bit := uint32(1)
	if zero {
		bit = 0
	}
	node, e := p.scalarBit(bit)
	if e != nil {
		return st, e
	}
	st = RewriteDone
	err = m.SetRef(out, node)
	return
}
func (p *BVRewriter) RewriteReductionAND(arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	// The source default r is constructed before is_numeral, even on FAILED.
	n, e := ast.NewBitVectorMinusOneNumerals(p.manager)
	p.minusOne = n
	if e != nil {
		return st, e
	}
	var width uint32
	numeral, e := n.Read(arg, &width)
	if e != nil {
		return st, e
	}
	if numeral {
		all, e := n.Compare(width)
		if e != nil {
			return st, e
		}
		bit := uint32(0)
		if all {
			bit = 1
		}
		node, e := p.scalarBit(bit)
		if e != nil {
			return st, e
		}
		st = RewriteDone
		if err = p.manager.SetRef(out, node); err != nil {
			return
		}
	}
	// AND's comparison expression ends after output assignment, followed by r.
	if err = n.SourceEnd(); err != nil {
		return
	}
	if err = n.Close(); err != nil {
		return
	}
	p.minusOne = nil
	return
}
func (p *BVRewriter) RewriteCompare(lhs, rhs ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	var result ast.ASTHandle
	var e error
	if lhs == rhs {
		result, e = p.scalarBit(1)
		if e != nil {
			return st, e
		}
		st = RewriteDone
	} else {
		first, e := m.IsBitVectorNumeral(lhs)
		if e != nil {
			return st, e
		}
		second := false
		if first {
			second, e = m.IsBitVectorNumeral(rhs)
			if e != nil {
				return st, e
			}
		}
		if first && second {
			// The source DEBUG inequality assertion is guaranteed by the outer guard.
			result, e = p.scalarBit(0)
			if e != nil {
				return st, e
			}
			st = RewriteDone
		} else {
			// C++ nested argument order is unspecified; Go builds equality, one, zero.
			eq, e := m.MkEq(lhs, rhs)
			if e != nil {
				return st, e
			}
			one, e := p.scalarBit(1)
			if e != nil {
				return st, e
			}
			zero, e := p.scalarBit(0)
			if e != nil {
				return st, e
			}
			result, e = m.MkIte(eq, one, zero)
			if e != nil {
				return st, e
			}
			st = Rewrite2
		}
	}
	err = m.SetRef(out, result)
	return
}
