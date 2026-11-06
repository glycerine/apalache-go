package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:436-584;
// src/ast/rewriter/bv_rewriter.h:82.

import (
	"github.com/glycerine/g3/pkg/ast"
	"reflect"
)

type bvLEInvocation struct {
	numerals *ast.BitVectorLEComparisonNumerals
	zeroBit  ast.ASTBitVectorNumeralInvocation
	operands [6]ast.ASTHandle
	header   uint64
}

func (v *bvLEInvocation) close(m *ast.ASTManager) error {
	if e := m.CleanupBitVectorNumeralInvocation(&v.zeroBit); e != nil {
		return e
	}
	if v.numerals != nil {
		if e := v.numerals.Close(); e != nil {
			return e
		}
		v.numerals = nil
	}
	m.Budget().Release(v.header)
	v.header = 0
	return nil
}
func (p *BVRewriter) RewriteLE(signed bool, a, b ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.rewriteLE(signed, a, b, out)
}
func (p *BVRewriter) rewriteLE(signed bool, a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	m := p.manager
	bytes := uint64(reflect.TypeOf(bvLEInvocation{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return RewriteFailed, e
	}
	v := &bvLEInvocation{header: bytes}
	p.leq = v
	var e error
	v.numerals, e = ast.NewBitVectorLEComparisonNumerals(m)
	if e != nil {
		return RewriteFailed, e
	}
	node, st, published, e := p.leBody(v, signed, a, b, out)
	if e != nil {
		return st, e
	}
	if st != RewriteFailed && !published {
		if e = m.SetRef(out, node); e != nil {
			return st, e
		}
	}
	if e = m.BitVectorNumeralInvocationSourceReady(&v.zeroBit); e != nil {
		return st, e
	}
	if e = v.numerals.SourceEnd(); e != nil {
		return st, e
	}
	if e = v.close(m); e != nil {
		return st, e
	}
	p.leq = nil
	return st, nil
}
func (p *BVRewriter) leBody(v *bvLEInvocation, signed bool, a, b ast.ASTHandle, out ast.ASTRef) (ast.ASTHandle, RewriteStatus, bool, error) {
	m := p.manager
	n := v.numerals
	failed := ast.ASTHandle{}
	var width uint32
	num1, e := n.Read(0, a, &width)
	if e != nil {
		return failed, RewriteFailed, false, e
	}
	num2, e := n.Read(1, b, &width)
	if e != nil {
		return failed, RewriteFailed, false, e
	}
	if a == b {
		node, e := m.MkTrue()
		return node, RewriteDone, false, e
	}
	if num1 {
		if e = n.Normalize(0, width, signed); e != nil {
			return failed, RewriteFailed, false, e
		}
	}
	if num2 {
		if e = n.Normalize(1, width, signed); e != nil {
			return failed, RewriteFailed, false, e
		}
	}
	if num1 && num2 {
		yes, e := n.CompareOperands()
		if e != nil {
			return failed, RewriteFailed, false, e
		}
		node, e := m.MkBoolVal(yes)
		return node, RewriteDone, false, e
	}
	if e = n.PlaceBounds(); e != nil {
		return failed, RewriteFailed, false, e
	}
	if num1 || num2 {
		if e = n.Bounds(width, signed); e != nil {
			return failed, RewriteFailed, false, e
		}
	}
	if num2 {
		equal, e := n.Equal(1, 3)
		if e != nil {
			return failed, RewriteFailed, false, e
		}
		if equal {
			node, e := m.MkEq(a, b)
			return node, Rewrite1, false, e
		}
		equal, e = n.Equal(1, 4)
		if e != nil {
			return failed, RewriteFailed, false, e
		}
		if equal {
			node, e := m.MkTrue()
			return node, RewriteDone, false, e
		}
	}
	if num1 {
		equal, e := n.Equal(0, 3)
		if e != nil {
			return failed, RewriteFailed, false, e
		}
		if equal {
			node, e := m.MkTrue()
			return node, RewriteDone, false, e
		}
		equal, e = n.Equal(0, 4)
		if e != nil {
			return failed, RewriteFailed, false, e
		}
		if equal {
			node, e := m.MkEq(a, b)
			return node, Rewrite1, false, e
		}
	}
	if signed && num2 {
		matched, e := m.BitVectorAddOperands(a, &v.operands[0], &v.operands[1])
		if e != nil {
			return failed, RewriteFailed, false, e
		}
		if matched {
			matched, e = m.BitVectorMulOperands(v.operands[1], &v.operands[2], &v.operands[3])
			if e != nil {
				return failed, RewriteFailed, false, e
			}
		}
		if matched {
			matched, e = n.Read(0, v.operands[2], &width)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
		}
		if matched {
			matched, e = n.MinusOneNormalized(width, signed)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
		}
		if matched {
			matched, e = m.BitVectorSRemIOperands(v.operands[3], &v.operands[4], &v.operands[5])
			if e != nil {
				return failed, RewriteFailed, false, e
			}
		}
		if matched {
			matched, e = n.Read(0, v.operands[5], &width)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
		}
		if matched {
			if e = n.Normalize(0, width, signed); e != nil {
				return failed, RewriteFailed, false, e
			}
			matched, e = n.IsPositive(0)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
		}
		if matched {
			matched, e = n.IsPositive(1)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
		}
		if matched {
			matched, e = n.RemainderIsZero()
			if e != nil {
				return failed, RewriteFailed, false, e
			}
		}
		if matched {
			matched, e = n.SumFitsSigned(width)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
		}
		if matched {
			if e = n.SumMinusOne(); e != nil {
				return failed, RewriteFailed, false, e
			}
			c, e := n.Constant(8, width)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
			node, e := m.MkBitVectorSLE(v.operands[0], c)
			return node, Rewrite2, false, e
		}
	}
	if p.polynomial.config.bitVector.LEExtra {
		st, e := p.rewriteLEConcat(signed, a, b, out)
		if e != nil {
			return failed, st, true, e
		}
		if st != RewriteFailed {
			return failed, st, true, p.traceLEExtra(514, signed, a, b, out)
		}
	}
	if p.polynomial.config.bitVector.LEExtra {
		st, e := p.rewriteLEOverflow(signed, a, b, out)
		if e != nil {
			return failed, st, true, e
		}
		if st != RewriteFailed {
			return failed, st, true, p.traceLEExtra(523, signed, a, b, out)
		}
	}
	if !signed {
		size, e := m.BitVectorSize(b)
		if e != nil {
			return failed, RewriteFailed, false, e
		}
		first := ^uint32(0)
		for i := size; i > 0; {
			i--
			zero, e := m.BitVectorZeroBitWithInvocation(b, i, &v.zeroBit)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
			if e = m.BitVectorNumeralInvocationSourceReady(&v.zeroBit); e != nil {
				return failed, RewriteFailed, false, e
			}
			if !zero {
				first = i
				break
			}
		}
		if first == ^uint32(0) {
			c, e := n.ZeroConstant(size)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
			node, e := m.MkEq(a, c)
			return node, Rewrite1, false, e
		} else if first < size-1 {
			hi, e := p.extract.Apply(size-1, first+1, a)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
			c, e := n.ZeroConstant(size - first - 1)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
			eq, e := m.MkEq(hi, c)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
			loA, e := p.extract.Apply(first, 0, a)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
			loB, e := p.extract.Apply(first, 0, b)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
			le, e := m.MkBitVectorULE(loA, loB)
			if e != nil {
				return failed, RewriteFailed, false, e
			}
			node, e := m.MkAndArgs([]ast.ASTHandle{eq, le})
			return node, Rewrite3, false, e
		}
	}
	return failed, RewriteFailed, false, nil
}
