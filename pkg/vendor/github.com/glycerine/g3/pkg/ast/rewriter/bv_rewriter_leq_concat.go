package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:245-252,349-434;
// src/ast/rewriter/bv_rewriter.h:79.

import (
	"github.com/glycerine/g3/pkg/ast"
	"reflect"
)

type bvLEConcatInvocation struct {
	numerals *ast.BitVectorConcatComparisonNumerals
	refs     [2]ast.ASTRef
	args     []ast.ASTHandle
	header   uint64
}

func (v *bvLEConcatInvocation) endRefs(m *ast.ASTManager) error {
	for i := 1; i >= 0; i-- {
		if v.refs[i] != (ast.ASTRef{}) {
			if e := m.DelRef(&v.refs[i]); e != nil {
				return e
			}
		}
	}
	return nil
}
func (v *bvLEConcatInvocation) close(m *ast.ASTManager) error {
	if v.numerals != nil {
		if e := v.numerals.CloseExpression(); e != nil {
			return e
		}
	}
	clear(v.args)
	v.args = nil
	if e := v.endRefs(m); e != nil {
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
func (p *BVRewriter) shortConcat(args ast.AppArguments) (ast.ASTHandle, error) {
	if ast.ASTSourceDebug && args.Count() == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if args.Count() == 1 {
		return args.At(0)
	}
	return p.manager.MkBitVectorConcatArguments(args)
}
func (p *BVRewriter) RewriteLEConcat(signed bool, a, b ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.rewriteLEConcat(signed, a, b, out)
}
func (p *BVRewriter) rewriteLEConcat(signed bool, a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	m := p.manager
	yes, e := m.IsBitVectorOperation(a, 37)
	if e != nil || !yes {
		return RewriteFailed, e
	}
	yes, e = m.IsBitVectorOperation(b, 37)
	if e != nil || !yes {
		return RewriteFailed, e
	}
	aa, e := m.AppArgumentView(a)
	if e != nil {
		return RewriteFailed, e
	}
	bb, e := m.AppArgumentView(b)
	if e != nil {
		return RewriteFailed, e
	}
	bytes := uint64(reflect.TypeOf(bvLEConcatInvocation{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return RewriteFailed, e
	}
	v := &bvLEConcatInvocation{header: bytes}
	p.leConcat = v
	node, st, e := p.leConcatBody(v, signed, aa, bb)
	if e != nil {
		return st, e
	}
	if st != RewriteFailed {
		if e = m.SetRef(out, node); e != nil {
			return st, e
		}
	}
	if e = v.endRefs(m); e != nil {
		return st, e
	}
	if v.numerals != nil {
		if e = v.numerals.SourceEnd(); e != nil {
			return st, e
		}
	}
	if e = v.close(m); e != nil {
		return st, e
	}
	p.leConcat = nil
	return st, nil
}
func (p *BVRewriter) leConcatBody(v *bvLEConcatInvocation, signed bool, a, b ast.AppArguments) (ast.ASTHandle, RewriteStatus, error) {
	m := p.manager
	failed := ast.ASTHandle{}
	numa, numb := a.Count(), b.Count()
	minimum := numa
	if numb < minimum {
		minimum = numb
	}
	if numa != 0 && numb != 0 {
		var e error
		v.numerals, e = ast.NewBitVectorConcatComparisonNumerals(m)
		if e != nil {
			return failed, RewriteFailed, e
		}
		af, e := a.At(0)
		if e != nil {
			return failed, RewriteFailed, e
		}
		var as, bs uint32
		numeric, e := v.numerals.Read(0, af, &as)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if numeric {
			bf, e := b.At(0)
			if e != nil {
				return failed, RewriteFailed, e
			}
			numeric, e = v.numerals.Read(1, bf, &bs)
			if e != nil {
				return failed, RewriteFailed, e
			}
		}
		if numeric {
			small := as
			if bs < small {
				small = bs
			}
			if e = v.numerals.High(0, as, small, signed); e != nil {
				return failed, RewriteFailed, e
			}
			if e = v.numerals.High(1, bs, small, signed); e != nil {
				return failed, RewriteFailed, e
			}
			equal, less, e := v.numerals.CompareHigh()
			if e != nil {
				return failed, RewriteFailed, e
			}
			if !equal {
				node, e := m.MkBoolVal(less)
				return node, RewriteDone, e
			}
			// Both owning defaults precede either branch's lexical argument buffer.
			for i := range v.refs {
				v.refs[i], e = m.NewRef(ast.ASTHandle{})
				if e != nil {
					return failed, RewriteFailed, e
				}
			}
			for i, old := range []ast.AppArguments{a, b} {
				width := as
				if i == 1 {
					width = bs
				}
				var node ast.ASTHandle
				if width > small {
					var head ast.ASTHandle
					head, e = v.numerals.Constant(i, width-small)
					if e != nil {
						return failed, RewriteFailed, e
					}
					v.args = append(v.args, head)
					for j := uint32(1); j < old.Count(); j++ {
						h, e := old.At(j)
						if e == nil {
							v.args = append(v.args, h)
						}
						if e != nil {
							return failed, RewriteFailed, e
						}
					}
					node, e = p.shortConcat(ast.SliceArguments(v.args))
				} else {
					var suffix ast.AppArguments
					suffix, e = old.Range(1, old.Count()-1)
					if e == nil {
						node, e = p.shortConcat(suffix)
					}
				}
				if e != nil {
					return failed, RewriteFailed, e
				}
				if e = m.SetRef(v.refs[i], node); e != nil {
					return failed, RewriteFailed, e
				}
				clear(v.args)
				v.args = nil
			}
			left, e := m.RefGet(v.refs[0])
			if e != nil {
				return failed, RewriteFailed, e
			}
			right, e := m.RefGet(v.refs[1])
			if e != nil {
				return failed, RewriteFailed, e
			}
			node, e := m.MkBitVectorULE(left, right)
			return node, Rewrite2, e
		}
		// The first-numeral lexical scope ends before prefix/postfix comparisons.
		if e = v.numerals.SourceEnd(); e != nil {
			return failed, RewriteFailed, e
		}
		if e = v.numerals.Close(); e != nil {
			return failed, RewriteFailed, e
		}
		v.numerals = nil
	}
	var common uint32
	for common < minimum {
		x, e := a.At(common)
		if e != nil {
			return failed, RewriteFailed, e
		}
		y, e := b.At(common)
		if e != nil {
			return failed, RewriteFailed, e
		}
		same, e := m.AreEqual(x, y)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if !same {
			break
		}
		common++
	}
	if ast.ASTSourceDebug && ((common == numa) != (common == numb)) {
		return failed, RewriteFailed, ast.ErrASTDomain
	}
	if common == numa {
		if ast.ASTSourceDebug {
			return failed, RewriteFailed, ast.ErrASTDomain
		}
		node, e := m.MkTrue()
		return node, RewriteDone, e
	}
	if common > 0 {
		aa, e := a.Range(common, numa-common)
		if e != nil {
			return failed, RewriteFailed, e
		}
		left, e := p.shortConcat(aa)
		if e != nil {
			return failed, RewriteFailed, e
		}
		bb, e := b.Range(common, numb-common)
		if e != nil {
			return failed, RewriteFailed, e
		}
		right, e := p.shortConcat(bb)
		if e != nil {
			return failed, RewriteFailed, e
		}
		node, e := m.MkBitVectorULE(left, right)
		return node, Rewrite2, e
	}
	na, nb := numa, numb
	for na != 0 && nb != 0 {
		x, e := a.At(na - 1)
		if e != nil {
			return failed, RewriteFailed, e
		}
		y, e := b.At(nb - 1)
		if e != nil {
			return failed, RewriteFailed, e
		}
		same, e := m.AreEqual(x, y)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if !same {
			break
		}
		na--
		nb--
	}
	if na == 0 {
		if ast.ASTSourceDebug {
			return failed, RewriteFailed, ast.ErrASTDomain
		}
		node, e := m.MkTrue()
		return node, RewriteDone, e
	}
	if na != numa {
		aa, e := a.Range(0, na)
		if e != nil {
			return failed, RewriteFailed, e
		}
		left, e := p.shortConcat(aa)
		if e != nil {
			return failed, RewriteFailed, e
		}
		bb, e := b.Range(0, nb)
		if e != nil {
			return failed, RewriteFailed, e
		}
		right, e := p.shortConcat(bb)
		if e != nil {
			return failed, RewriteFailed, e
		}
		var node ast.ASTHandle
		if signed {
			node, e = m.MkBitVectorSLE(left, right)
		} else {
			node, e = m.MkBitVectorULE(left, right)
		}
		return node, Rewrite2, e
	}
	return failed, RewriteFailed, nil
}
