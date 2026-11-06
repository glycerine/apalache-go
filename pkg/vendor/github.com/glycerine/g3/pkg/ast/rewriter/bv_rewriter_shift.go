package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:792-889.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteShiftLeft(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteLogicalShift(lhs, rhs, out, true)
}
func (p *BVRewriter) RewriteLogicalShiftRight(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteLogicalShift(lhs, rhs, out, false)
}
func (p *BVRewriter) rewriteLogicalShift(lhs, rhs ast.ASTHandle, out ast.ASTRef, left bool) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	n, e := ast.NewBitVectorLogicalShiftNumerals(m)
	p.shift = n
	if e != nil {
		return st, e
	}
	width, e := m.BitVectorSize(lhs)
	if e != nil {
		return st, e
	}
	var storedWidth uint32
	yes, e := n.Read(1, rhs, &storedWidth)
	if e != nil {
		return st, e
	}
	if yes {
		zero, e := n.ZeroAmount()
		if e != nil {
			return st, e
		}
		result := lhs
		st = RewriteDone
		if !zero {
			large, e := n.AtLeastWidth(width)
			if e != nil {
				return st, e
			}
			if large {
				result, e = n.Zero(width)
				if e != nil {
					return st, e
				}
			} else {
				numeric, e := n.Read(0, lhs, &storedWidth)
				if e != nil {
					return st, e
				}
				if numeric {
					slot := 0
					if width <= 64 {
						if e = n.Narrow(width, left); e != nil {
							return st, e
						}
						slot = 3
					} else if e = n.Wide(width, left); e != nil {
						return st, e
					}
					result, e = n.Constant(slot, width)
					if e != nil {
						return st, e
					}
				} else {
					shift, e := n.Amount(width, left, true)
					if e != nil {
						return st, e
					}
					// The original initializer is ordered: left extract then zero; right
					// zero then extract. Both raw handles live through concat publication.
					if left {
						p.shiftArgs[0], e = p.extract.Apply(width-shift-1, 0, lhs)
						if e != nil {
							return st, e
						}
						p.shiftArgs[1], e = n.Zero(shift)
						if e != nil {
							return st, e
						}
					} else {
						p.shiftArgs[0], e = n.Zero(shift)
						if e != nil {
							return st, e
						}
						p.shiftArgs[1], e = p.extract.Apply(width-1, shift, lhs)
						if e != nil {
							return st, e
						}
					}
					result, e = m.MkBitVectorConcat(p.shiftArgs[:])
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
	}
	if err = n.SourceEnd(); err != nil {
		return
	}
	if err = n.Close(); err != nil {
		return
	}
	p.shift = nil
	return
}
