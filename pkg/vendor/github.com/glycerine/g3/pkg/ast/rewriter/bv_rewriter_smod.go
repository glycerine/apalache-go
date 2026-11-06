package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1282-1341;
// src/ast/rewriter/bv_rewriter.h:125,130.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteSignedModulo(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	return p.rewriteSignedModulo(lhs, rhs, out, p.polynomial.config.bitVector.HiDiv0)
}
func (p *BVRewriter) RewriteSignedModuloInternal(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteSignedModulo(lhs, rhs, out, true)
}
func (p *BVRewriter) rewriteSignedModulo(lhs, rhs ast.ASTHandle, out ast.ASTRef, hi bool) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	n, e := ast.NewBitVectorSignedModuloNumerals(m)
	p.signedModulo = n
	if e != nil {
		return st, e
	}
	var width uint32
	first, e := n.Read(0, lhs, &width)
	if e != nil {
		return st, e
	}
	var result ast.ASTHandle
	if first {
		if e = n.Normalize(0, width); e != nil {
			return st, e
		}
		zero, e := n.IsZero(0)
		if e != nil {
			return st, e
		}
		if zero {
			result, e = m.MkFamilyApp(p.bvFamily, 10, nil, []ast.ASTHandle{lhs, rhs})
			if e != nil {
				return st, e
			}
			st = Rewrite1
		}
	}
	if st == RewriteFailed {
		second, e := n.Read(1, rhs, &width)
		if e != nil {
			return st, e
		}
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
				} else {
					result, e = m.MkFamilyApp(p.bvFamily, 16, nil, []ast.ASTHandle{lhs})
					if e != nil {
						return st, e
					}
				}
				st = RewriteDone
			} else if first {
				if e = n.Modulo(width); e != nil {
					return st, e
				}
				result, e = n.Constant(6, width)
				if e != nil {
					return st, e
				}
				st = RewriteDone
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
					st = Rewrite2
				}
			}
		}
		if st == RewriteFailed {
			if hi {
				result, e = m.MkFamilyApp(p.bvFamily, 21, nil, []ast.ASTHandle{lhs, rhs})
				if e != nil {
					return st, e
				}
				st = RewriteDone
			} else {
				width, e = m.BitVectorSize(rhs)
				if e != nil {
					return st, e
				}
				zero, e := n.Zero(width)
				if e != nil {
					return st, e
				}
				cond, e := m.MkEq(rhs, zero)
				if e != nil {
					return st, e
				}
				yes, e := m.MkFamilyApp(p.bvFamily, 16, nil, []ast.ASTHandle{lhs})
				if e != nil {
					return st, e
				}
				no, e := m.MkFamilyApp(p.bvFamily, 21, nil, []ast.ASTHandle{lhs, rhs})
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
	p.signedModulo = nil
	return
}
