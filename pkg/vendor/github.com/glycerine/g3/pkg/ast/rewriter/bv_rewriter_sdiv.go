package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:987-1032;
// src/ast/rewriter/bv_rewriter.h:121,126.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteSignedDivision(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	return p.rewriteSignedDivision(lhs, rhs, out, p.polynomial.config.bitVector.HiDiv0)
}
func (p *BVRewriter) RewriteSignedDivisionInternal(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteSignedDivision(lhs, rhs, out, true)
}
func (p *BVRewriter) rewriteSignedDivision(lhs, rhs ast.ASTHandle, out ast.ASTRef, hiDiv0 bool) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	n, e := ast.NewBitVectorSignedDivisionNumerals(m)
	p.signedDivision = n
	if e != nil {
		return st, e
	}
	var width uint32
	num, e := n.Read(1, rhs, &width)
	if e != nil {
		return st, e
	}
	internal := func() (ast.ASTHandle, error) { return m.MkFamilyApp(p.bvFamily, 17, nil, []ast.ASTHandle{lhs, rhs}) }
	special := func() (ast.ASTHandle, error) { return m.MkFamilyApp(p.bvFamily, 12, nil, []ast.ASTHandle{lhs}) }
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
				// Preserve raw (ite (bvslt lhs zero) one allones) even for numeral lhs.
				// C++ nested argument evaluation order is unspecified; Go uses this order.
				zero, e := n.Unsigned(0, width)
				if e != nil {
					return st, e
				}
				cond, e := m.MkFamilyApp(p.bvFamily, 27, nil, []ast.ASTHandle{lhs, zero})
				if e != nil {
					return st, e
				}
				one, e := n.Unsigned(1, width)
				if e != nil {
					return st, e
				}
				if e = n.AllOnes(width); e != nil {
					return st, e
				}
				all, e := n.Constant(6, width)
				if e != nil {
					return st, e
				}
				result, e = m.MkIte(cond, one, all)
				if e != nil {
					return st, e
				}
				st = Rewrite2
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
				} else {
					result, e = internal()
					if e != nil {
						return st, e
					}
				}
				st = RewriteDone
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
	p.signedDivision = nil
	return
}
