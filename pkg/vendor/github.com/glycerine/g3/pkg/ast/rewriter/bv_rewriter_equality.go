package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2510-2635.

import "github.com/glycerine/g3/pkg/ast"

func bvUnsignedRemainderAny(m *ast.ASTManager, arg ast.ASTHandle) (yes bool, dividend, divisor ast.ASTHandle, err error) {
	yes, err = m.IsBitVectorOperation(arg, 10)
	if err != nil {
		return
	}
	if !yes {
		yes, err = m.IsBitVectorOperation(arg, 20)
		if err != nil || !yes {
			return
		}
	}
	if ast.ASTSourceDebug {
		var count uint32
		count, err = m.GetNumArgs(arg)
		if err != nil {
			return
		}
		if count != 2 {
			err = ast.ErrASTDomain
			return
		}
	}
	dividend, err = m.GetArg(arg, 0)
	if err != nil {
		return
	}
	divisor, err = m.GetArg(arg, 1)
	return
}
func (p *BVRewriter) endEqualityRefs() error {
	for i := 1; i >= 0; i-- {
		if p.eqRefs[i] != (ast.ASTRef{}) {
			if e := p.manager.DelRef(&p.eqRefs[i]); e != nil {
				return e
			}
		}
	}
	return nil
}
func (p *BVRewriter) RewriteEquality(lhs, rhs ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	m, b := p.manager, p.polynomial
	options := b.config.bitVector
	st = RewriteFailed
	assign := func(h ast.ASTHandle, status RewriteStatus) error { st = status; return m.SetRef(out, h) }
	eq := func(a, b ast.ASTHandle, status RewriteStatus) error {
		h, e := m.MkEq(a, b)
		if e != nil {
			return e
		}
		return assign(h, status)
	}
	boolean := func(value bool) error {
		var h ast.ASTHandle
		var e error
		if value {
			h, e = m.MkTrue()
		} else {
			h, e = m.MkFalse()
		}
		if e != nil {
			return e
		}
		return assign(h, RewriteDone)
	}
	isNum := func(arg ast.ASTHandle) (bool, error) { return m.IsBitVectorOperation(arg, 0) }
	if lhs == rhs {
		err = boolean(true)
		return
	}
	ln, e := isNum(lhs)
	if e != nil {
		return st, e
	}
	if ln {
		rn, e := isNum(rhs)
		if e != nil {
			return st, e
		}
		if rn {
			err = boolean(false)
			return
		}
	}
	// Source performs the lhs recognizer again before the swap.
	ln, e = isNum(lhs)
	if e != nil {
		return st, e
	}
	swapped := ln
	if swapped {
		lhs, rhs = rhs, lhs
	}
	if options.BitToBool {
		st, err = b.RewriteBit2Bool(lhs, rhs, out)
		if err != nil || st != RewriteFailed {
			return
		}
	}
	if options.Trailing {
		st, err = p.trailing.EqRemoveTrailing(lhs, rhs, out)
		if err != nil {
			return
		}
		err = p.trailing.ResetCache(1 << 12)
		if err != nil {
			return
		}
		if st != RewriteFailed {
			err = p.traceEqualityRewrite("eq_remove_trailing", 2548, lhs, rhs, out)
			return
		}
	}
	st, err = b.RewriteMultiplicationEquality(lhs, rhs, out)
	if err != nil {
		return
	}
	if st != RewriteFailed {
		err = p.traceEqualityRewrite("mk_mul_eq", 2555, lhs, rhs, out)
		return
	}
	st, err = b.RewriteMultiplicationEquality(rhs, lhs, out)
	if err != nil {
		return
	}
	if st != RewriteFailed {
		err = p.traceEqualityRewrite("mk_mul_eq", 2561, lhs, rhs, out)
		return
	}
	if options.BlastEqualityValue {
		st, err = b.RewriteBlastEqualityValue(lhs, rhs, out, &p.extract)
		if err != nil || st != RewriteFailed {
			return
		}
	}
	if options.RemainderSimplification {
		n, e := ast.NewBitVectorRemainderEqualityNumerals(m)
		p.eqRemainder = n
		if e != nil {
			return st, e
		}
		yes, dividend, divisor, e := bvUnsignedRemainderAny(m, lhs)
		if e != nil {
			return st, e
		}
		divisorWidth, rhsWidth := uint32(0), uint32(0)
		if yes {
			yes, e = n.Read(1, rhs, &rhsWidth)
			if e != nil {
				return st, e
			}
		}
		if yes {
			yes, e = n.Read(0, divisor, &divisorWidth)
			if e != nil {
				return st, e
			}
		}
		if yes {
			outside, e := n.Outside()
			if e != nil {
				return st, e
			}
			if outside {
				err = boolean(false)
			} else {
				reaches, e := n.ReachesModulus(divisorWidth)
				if e != nil {
					return st, e
				}
				if reaches {
					err = eq(dividend, rhs, Rewrite2)
				}
			}
			if err != nil {
				return
			}
		}
		if err = n.SourceEnd(); err != nil {
			return
		}
		if err = n.Close(); err != nil {
			return
		}
		p.eqRemainder = nil
		if st != RewriteFailed {
			return
		}
	}
	// Both default owning refs precede the polynomial guard, even when false.
	p.eqRefs[0], err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return
	}
	p.eqRefs[1], err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return
	}
	finish := func() (RewriteStatus, error) { return st, p.endEqualityRefs() }
	polynomial := false
	for _, operand := range [4]struct {
		arg ast.ASTHandle
		op  int32
	}{{lhs, 4}, {lhs, 6}, {rhs, 4}, {rhs, 6}} {
		yes, e := m.IsBitVectorOperation(operand.arg, operand.op)
		if e != nil {
			return st, e
		}
		if yes {
			polynomial = true
			break
		}
	}
	if polynomial {
		st, err = b.CancelMonomials(lhs, rhs, false, p.eqRefs[0], p.eqRefs[1])
		if err != nil {
			return
		}
		if st != RewriteFailed {
			l, e := m.RefGet(p.eqRefs[0])
			if e != nil {
				return st, e
			}
			yes, e := isNum(l)
			if e != nil {
				return st, e
			}
			if yes {
				r, e := m.RefGet(p.eqRefs[1])
				if e != nil {
					return st, e
				}
				yes, e = isNum(r)
				if e != nil {
					return st, e
				}
				if yes {
					err = boolean(l == r)
					if err != nil {
						return
					}
					return finish()
				}
			}
		} else {
			err = m.SetRef(p.eqRefs[0], lhs)
			if err != nil {
				return
			}
			err = m.SetRef(p.eqRefs[1], rhs)
			if err != nil {
				return
			}
		}
		lhs, err = m.RefGet(p.eqRefs[0])
		if err != nil {
			return
		}
		rhs, err = m.RefGet(p.eqRefs[1])
		if err != nil {
			return
		}
		matched, e := b.IsolateBitVectorTerm(lhs, rhs, out)
		if e != nil {
			return st, e
		}
		if matched {
			st = Rewrite2
			return finish()
		}
		target, e := IsBitVectorConcatEqualityTarget(m, lhs, rhs, options.SplitConcatEquality)
		if e != nil {
			return st, e
		}
		if target {
			st, err = b.RewriteConcatenationEquality(lhs, rhs, out, &p.extract)
			if err != nil {
				return
			}
			return finish()
		}
		if st != RewriteFailed {
			err = eq(lhs, rhs, RewriteDone)
			if err != nil {
				return
			}
			return finish()
		}
	}
	target, e := IsBitVectorConcatEqualityTarget(m, lhs, rhs, options.SplitConcatEquality)
	if e != nil {
		return st, e
	}
	if target {
		st, err = b.RewriteConcatenationEquality(lhs, rhs, out, &p.extract)
		if err != nil {
			return
		}
		return finish()
	}
	if swapped {
		err = eq(lhs, rhs, RewriteDone)
		if err != nil {
			return
		}
		return finish()
	}
	st = RewriteFailed
	return finish()
}
