package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1101-1144;
// src/ast/rewriter/bv_rewriter.h:123,128.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteSignedRemainder(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	return p.rewriteSignedRemainder(lhs, rhs, out, p.polynomial.config.bitVector.HiDiv0)
}
func (p *BVRewriter) RewriteSignedRemainderInternal(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteSignedRemainder(lhs, rhs, out, true)
}
func (p *BVRewriter) rewriteSignedRemainder(lhs, rhs ast.ASTHandle, out ast.ASTRef, hiDiv0 bool) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	n, e := ast.NewBitVectorSignedRemainderNumerals(m)
	p.signedRemainder = n
	if e != nil {
		return st, e
	}
	var width uint32
	num, e := n.Read(1, rhs, &width)
	if e != nil {
		return st, e
	}
	internal := func() (ast.ASTHandle, error) { return m.MkFamilyApp(p.bvFamily, 19, nil, []ast.ASTHandle{lhs, rhs}) }
	special := func() (ast.ASTHandle, error) { return m.MkFamilyApp(p.bvFamily, 14, nil, []ast.ASTHandle{lhs}) }
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
			if hiDiv0 {
				result = lhs
				st = RewriteDone
			} else {
				result, e = special()
				if e != nil {
					return st, e
				}
				st = Rewrite1
			}
		} else {
			one, e := n.IsOne(1)
			if e != nil {
				return st, e
			}
			if one {
				result, e = n.Zero(width)
				if e != nil {
					return st, e
				}
				st = RewriteDone
			} else {
				// Preserve the second reached !r2.is_zero() source query.
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
					if e = n.Remainder(); e != nil {
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
		// C++ nested arguments have unspecified evaluation order. This Go adapter
		// constructs condition, then zero interpretation, then internal operation.
		zero, e := n.Zero(width)
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
	p.signedRemainder = nil
	return
}
