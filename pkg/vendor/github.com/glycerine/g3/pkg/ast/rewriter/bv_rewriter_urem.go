package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1185-1280;
// src/ast/rewriter/bv_rewriter.h:124,129.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteUnsignedRemainder(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	return p.rewriteUnsignedRemainder(lhs, rhs, out, p.polynomial.config.bitVector.HiDiv0)
}
func (p *BVRewriter) RewriteUnsignedRemainderInternal(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteUnsignedRemainder(lhs, rhs, out, true)
}
func (p *BVRewriter) rewriteUnsignedRemainder(lhs, rhs ast.ASTHandle, out ast.ASTRef, hi bool) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	n, e := ast.NewBitVectorUnsignedRemainderNumerals(m)
	p.unsignedRemainder = n
	if e != nil {
		return st, e
	}
	var width uint32
	first, e := n.Read(0, lhs, &width)
	if e != nil {
		return st, e
	}
	second, e := n.Read(1, rhs, &width)
	if e != nil {
		return st, e
	}
	special := func(a ast.ASTHandle) (ast.ASTHandle, error) {
		return m.MkFamilyApp(p.bvFamily, 15, nil, []ast.ASTHandle{a})
	}
	internal := func() (ast.ASTHandle, error) { return m.MkFamilyApp(p.bvFamily, 20, nil, []ast.ASTHandle{lhs, rhs}) }
	var result ast.ASTHandle
	if second {
		if e = n.Normalize(1, width); e != nil {
			return st, e
		}
		zero, e := n.IsZero(1)
		if e != nil {
			return st, e
		}
		if zero {
			if hi {
				result = lhs
				st = RewriteDone
			} else {
				result, e = special(lhs)
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
				result, e = n.Unsigned(0, width)
				if e != nil {
					return st, e
				}
				st = RewriteDone
			} else {
				zero, e = n.IsZero(1)
				if e != nil {
					return st, e
				}
				if !zero && first {
					if e = n.Normalize(0, width); e != nil {
						return st, e
					}
					if e = n.Remainder(); e != nil {
						return st, e
					}
					result, e = n.Constant(0, width)
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
						p.uremArgs[0], e = n.Unsigned(0, width-shift)
						if e != nil {
							return st, e
						}
						p.uremArgs[1], e = p.extract.Apply(shift-1, 0, lhs)
						if e != nil {
							return st, e
						}
						result, e = m.MkBitVectorConcat(p.uremArgs[:])
						if e != nil {
							return st, e
						}
						st = Rewrite2
					} else {
						result, e = internal()
						if e != nil {
							return st, e
						}
						st = RewriteDone
					}
				}
			}
		}
	} else {
		zero := false
		if first {
			zero, e = n.IsZero(0)
			if e != nil {
				return st, e
			}
		}
		if zero {
			if hi {
				result = lhs
				st = RewriteDone
			} else {
				cond, e := m.MkEq(rhs, lhs)
				if e != nil {
					return st, e
				}
				yes, e := special(lhs)
				if e != nil {
					return st, e
				}
				result, e = m.MkIte(cond, yes, lhs)
				if e != nil {
					return st, e
				}
				st = Rewrite2
			}
		} else {
			var x ast.ASTHandle
			match, e := p.isXMinusOne(lhs, &x)
			if e != nil {
				return st, e
			}
			if match && x == rhs {
				if hi {
					result = lhs
					st = RewriteDone
				} else {
					width, e = m.BitVectorSize(lhs)
					if e != nil {
						return st, e
					}
					if e = n.AllOnes(width); e != nil {
						return st, e
					}
					minus, e := n.Constant(5, width)
					if e != nil {
						return st, e
					}
					// End the minus_one declaration's full expression before building ITE.
					if e = n.End(5, 4, 3); e != nil {
						return st, e
					}
					zero, e := n.Unsigned(0, width)
					if e != nil {
						return st, e
					}
					cond, e := m.MkEq(x, zero)
					if e != nil {
						return st, e
					}
					yes, e := special(minus)
					if e != nil {
						return st, e
					}
					result, e = m.MkIte(cond, yes, lhs)
					if e != nil {
						return st, e
					}
					st = Rewrite2
				}
			} else if hi {
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
				// Go evaluates these nested C++ arguments as condition, yes, then no.
				zero, e := n.Unsigned(0, width)
				if e != nil {
					return st, e
				}
				cond, e := m.MkEq(rhs, zero)
				if e != nil {
					return st, e
				}
				yes, e := special(lhs)
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
	p.unsignedRemainder = nil
	return
}
