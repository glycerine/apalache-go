package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:891-985.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteArithmeticShiftRight(lhs, rhs ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	n, e := ast.NewBitVectorArithmeticShiftNumerals(m)
	p.arithmeticShift = n
	if e != nil {
		return st, e
	}
	width, e := m.BitVectorSize(lhs)
	if e != nil {
		return st, e
	}
	if ast.ASTSourceDebug && width == 0 {
		return st, ast.ErrASTDomain
	}
	num2, e := n.Read(1, rhs, &width)
	if e != nil {
		return st, e
	}
	if num2 {
		zero, e := n.ZeroAmount()
		if e != nil {
			return st, e
		}
		if zero {
			st = RewriteDone
			if err = m.SetRef(out, lhs); err != nil {
				return
			}
			return p.endArithmeticShift(st)
		}
	}
	num1, e := n.Read(0, lhs, &width)
	if e != nil {
		return st, e
	}
	if num1 && num2 {
		var result ast.ASTHandle
		if width <= 64 {
			if e = n.Narrow(width); e != nil {
				return st, e
			}
			result, e = n.Constant(3, width)
			if e != nil {
				return st, e
			}
		} else {
			large, e := n.AtLeastWidth(width)
			if e != nil {
				return st, e
			}
			if large {
				sign, e := n.HasSign(width)
				if e != nil {
					return st, e
				}
				if sign {
					if e = n.AllOnes(width); e != nil {
						return st, e
					}
					result, e = n.Constant(5, width)
				} else {
					result, e = n.Zero(width)
				}
				if e != nil {
					return st, e
				}
			} else {
				if e = n.Wide(width); e != nil {
					return st, e
				}
				result, e = n.Constant(0, width)
				if e != nil {
					return st, e
				}
			}
		}
		st = RewriteDone
		if err = m.SetRef(out, result); err != nil {
			return
		}
		return p.endArithmeticShift(st)
	}
	if num2 {
		nested, e := m.IsBitVectorOperation(lhs, 47)
		if e != nil {
			return st, e
		}
		if nested {
			args, e := m.AppArgumentView(lhs)
			if e != nil {
				return st, e
			}
			amount, e := args.At(1)
			if e != nil {
				return st, e
			}
			numeral, e := n.Read(0, amount, &width)
			if e != nil {
				return st, e
			}
			if numeral {
				if e = n.CombineAndClamp(width); e != nil {
					return st, e
				}
				base, e := args.At(0)
				if e != nil {
					return st, e
				}
				amount, e = n.Constant(0, width)
				if e != nil {
					return st, e
				}
				result, e := m.MkBitVectorASHR(base, amount)
				if e != nil {
					return st, e
				}
				st = Rewrite1
				if err = m.SetRef(out, result); err != nil {
					return
				}
			}
		}
	}
	// The symbolic sign-extraction rewrite is disabled by the original #if 0.
	return p.endArithmeticShift(st)
}
func (p *BVRewriter) endArithmeticShift(st RewriteStatus) (RewriteStatus, error) {
	n := p.arithmeticShift
	if e := n.SourceEnd(); e != nil {
		return st, e
	}
	if e := n.Close(); e != nil {
		return st, e
	}
	p.arithmeticShift = nil
	return st, nil
}
