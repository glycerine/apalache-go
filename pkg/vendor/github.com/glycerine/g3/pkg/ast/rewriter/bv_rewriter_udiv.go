package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1034-1099;
// src/ast/rewriter/bv_rewriter.h:122,127.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteUnsignedDivision(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	return p.rewriteUnsignedDivision(lhs, rhs, out, p.polynomial.config.bitVector.HiDiv0)
}
func (p *BVRewriter) RewriteUnsignedDivisionInternal(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteUnsignedDivision(lhs, rhs, out, true)
}
func (p *BVRewriter) rewriteUnsignedDivision(lhs, rhs ast.ASTHandle, out ast.ASTRef, hiDiv0 bool) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	p.traceUnsignedDivisionEntry(hiDiv0)
	if err = p.traceUnsignedDivisionOption(rhs); err != nil {
		return st, err
	}
	n, e := ast.NewBitVectorUnsignedDivisionNumerals(m)
	p.unsignedDivision = n
	if e != nil {
		return st, e
	}
	var width uint32
	num, e := n.Read(1, rhs, &width)
	if e != nil {
		return st, e
	}
	internal := func() (ast.ASTHandle, error) { return m.MkFamilyApp(p.bvFamily, 18, nil, []ast.ASTHandle{lhs, rhs}) }
	special := func() (ast.ASTHandle, error) { return m.MkFamilyApp(p.bvFamily, 13, nil, []ast.ASTHandle{lhs}) }
	var result ast.ASTHandle
	if num {
		if e = n.Normalize(1, width); e != nil {
			return st, e
		}
		zero, e := n.IsZero(1)
		if e != nil {
			return st, e
		}
		if zero {
			if !hiDiv0 {
				result, e = special()
				if e != nil {
					return st, e
				}
				st = Rewrite1
			} else {
				if e = n.AllOnes(width); e != nil {
					return st, e
				}
				result, e = n.Constant(6, width)
				if e != nil {
					return st, e
				}
				st = RewriteDone
			}
		} else {
			one, e := n.IsOne(1)
			if e != nil {
				return st, e
			}
			if one {
				result = lhs
				st = RewriteDone
			} else {
				zero, e = n.IsZero(1)
				if e != nil {
					return st, e
				}
				matched := false
				if !zero {
					matched, e = n.Read(0, lhs, &width)
					if e != nil {
						return st, e
					}
				}
				if matched {
					if e = n.Normalize(0, width); e != nil {
						return st, e
					}
					if e = n.Quotient(); e != nil {
						return st, e
					}
					result, e = n.Constant(3, width)
					if e != nil {
						return st, e
					}
					st = RewriteDone
				} else {
					power, shift, e := n.PowerOfTwo()
					if e != nil {
						return st, e
					}
					if power {
						amount, e := n.Unsigned(shift, width)
						if e != nil {
							return st, e
						}
						result, e = m.MkBitVectorLSHR(lhs, amount)
						if e != nil {
							return st, e
						}
						st = Rewrite1
					} else {
						if p.polynomial.config.bitVector.UnsignedDivisionToMultiply {
							p.traceUnsignedDivisionToMultiply()
							invertible, e := n.Inverse(width)
							if e != nil {
								return st, e
							}
							if invertible {
								coefficient, e := n.Constant(7, width)
								if e != nil {
									return st, e
								}
								result, e = m.MkBitVectorMul(coefficient, lhs)
								if e != nil {
									return st, e
								}
								st = Rewrite1
							} else if e = n.End(7); e != nil {
								return st, e
							}
						}
						if st == RewriteFailed {
							result, e = internal()
							if e != nil {
								return st, e
							}
							st = RewriteDone
						}
					}
				}
			}
		}
	} else if hiDiv0 {
		result, e = internal()
		if e != nil {
			return st, e
		}
		st = RewriteDone
	} else {
		width, e = m.BitVectorSize(rhs)
		if e != nil {
			return st, e
		}
		// C++ nested arguments have unspecified evaluation order. Go builds the
		// condition, zero interpretation and internal operation in this order.
		zero, e := n.Unsigned(0, width)
		if e != nil {
			return st, e
		}
		cond, e := m.MkEq(rhs, zero)
		if e != nil {
			return st, e
		}
		yes, e := special()
		if e != nil {
			return st, e
		}
		no, e := internal()
		if e != nil {
			return st, e
		}
		result, e = m.MkIte(cond, yes, no)
		if e != nil {
			return st, e
		}
		st = Rewrite2
		if e = p.traceUnsignedDivisionRewrite(lhs, rhs, result); e != nil {
			return st, e
		}
	}
	if err = m.SetRef(out, result); err != nil {
		return
	}
	if err = n.SourceEnd(); err != nil {
		return
	}
	if err = n.Close(); err != nil {
		return
	}
	p.unsignedDivision = nil
	return
}
