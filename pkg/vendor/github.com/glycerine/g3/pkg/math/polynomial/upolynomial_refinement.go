package polynomial

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *UPolynomialManager) isolationBinary(b *unumeral.BinaryRationalManager) error {
	if e := m.check(); e != nil {
		return e
	}
	if b == nil || b.Integers() != m.config.Integer {
		return ErrUPolynomialOwner
	}
	return b.Open()
}
func (m *UPolynomialManager) IsolatingToRefinable(v UPolynomialView, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational) (bool, error) {
	return m.isolatingToRefinableWithReceipt(v, bqm, a, b, nil)
}
func (m *UPolynomialManager) isolatingToRefinableWithReceipt(v UPolynomialView, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational, receipt *upolyRefinableInvocationReceipt) (result bool, cause error) {
	if e := m.isolationBinary(bqm); e != nil {
		return false, e
	}
	q, e := m.beginRefinable(bqm, receipt)
	if e != nil {
		return false, e
	}
	defer func() { cause = q.finish(cause) }()
	ops := bqm.WithAllocationContext(unumeral.NumericAllocationContext{}.WithCaller(&q.hooks))
	sa, e := q.eval(v, a)
	if e != nil {
		return false, e
	}
	sb, e := q.eval(v, b)
	if e != nil {
		return false, e
	}
	if e = q.traceSigns(sa, sb); e != nil {
		return false, e
	}
	if sa != 0 && sb != 0 {
		if e := m.refinableSignAssertion(2605, sa, sb); e != nil {
			return false, e
		}
		return true, nil
	}
	if sa == 0 && sb != 0 {
		next := q.construct()
		if e = ops.AddSub(a, b, next, false); e != nil {
			return false, e
		}
		if e = ops.Div2(next); e != nil {
			return false, e
		}
		for {
			if e = q.traceInterval(2615, [6]*unumeral.BinaryRational{a, b, next}); e != nil {
				return false, e
			}
			sn, e := q.eval(v, next)
			if e != nil {
				return false, e
			}
			if sn != sb {
				if e = bqm.Swap(next, a); e != nil {
					return false, e
				}
				if e := m.refinableSignAssertion(2619, sn, sb); e != nil {
					return false, e
				}
				return sn != 0, nil
			}
			if e := m.refinableSignAssertion(2624, sn, sb); e != nil {
				return false, e
			}
			if e = bqm.Swap(b, next); e != nil {
				return false, e
			}
			if e = ops.AddSub(b, a, next, false); e != nil {
				return false, e
			}
			if e = ops.Div2(next); e != nil {
				return false, e
			}
		}
	}
	if sa != 0 && sb == 0 {
		next := q.construct()
		if e = ops.AddSub(a, b, next, false); e != nil {
			return false, e
		}
		if e = ops.Div2(next); e != nil {
			return false, e
		}
		for {
			if e = q.traceInterval(2640, [6]*unumeral.BinaryRational{a, b, next}); e != nil {
				return false, e
			}
			sn, e := q.eval(v, next)
			if e != nil {
				return false, e
			}
			if sn != sa {
				if e := m.refinableSignAssertion(2643, sn, sa); e != nil {
					return false, e
				}
				if sn == 0 {
					e = bqm.Swap(next, a)
				} else {
					e = bqm.Swap(next, b)
				}
				if e != nil {
					return false, e
				}
				return sn != 0, nil
			}
			if e := m.refinableSignAssertion(2652, sn, sa); e != nil {
				return false, e
			}
			if e = bqm.Swap(a, next); e != nil {
				return false, e
			}
			if e = ops.AddSub(b, a, next, false); e != nil {
				return false, e
			}
			if e = ops.Div2(next); e != nil {
				return false, e
			}
		}
	}
	if e := m.refinableSignAssertion(2661, sa, sb); e != nil {
		return false, e
	}
	b1, a2, newA, newB := q.construct(), q.construct(), q.construct(), q.construct()
	if e = ops.AddSub(a, b, b1, false); e != nil {
		return false, e
	}
	if e = ops.Div2(b1); e != nil {
		return false, e
	}
	if e = bqm.CopyWithAllocationReceipt(a2, b1, &q.copy); e != nil {
		return false, e
	}
	s1, e := q.eval(v, b1)
	if e != nil {
		return false, e
	}
	s2 := s1
	if s1 == 0 {
		if e = bqm.Swap(b1, a); e != nil {
			return false, e
		}
		return false, nil
	}
	if e = ops.AddSub(a, b1, newA, false); e != nil {
		return false, e
	}
	if e = ops.Div2(newA); e != nil {
		return false, e
	}
	if e = ops.AddSub(a2, b, newB, false); e != nil {
		return false, e
	}
	if e = ops.Div2(newB); e != nil {
		return false, e
	}
	for {
		if e = q.traceInterval(2693, [6]*unumeral.BinaryRational{a, b1, newA, a2, b, newB}); e != nil {
			return false, e
		}
		snA, e := q.eval(v, newA)
		if e != nil {
			return false, e
		}
		if snA == 0 {
			if e = bqm.Swap(newA, a); e != nil {
				return false, e
			}
			return false, nil
		}
		if snA == -s1 {
			if e = bqm.Swap(newA, a); e != nil {
				return false, e
			}
			if e = bqm.Swap(b1, b); e != nil {
				return false, e
			}
			return true, nil
		}
		snB, e := q.eval(v, newB)
		if e != nil {
			return false, e
		}
		if snB == 0 {
			if e = bqm.Swap(newB, a); e != nil {
				return false, e
			}
			return false, nil
		}
		if snB == -s2 {
			if e = bqm.Swap(a2, a); e != nil {
				return false, e
			}
			if e = bqm.Swap(newB, b); e != nil {
				return false, e
			}
			return true, nil
		}
		if e := m.refinableSignAssertion(2727, snA, s1); e != nil {
			return false, e
		}
		if e = bqm.Swap(b1, newA); e != nil {
			return false, e
		}
		if e = ops.AddSub(b1, a, newA, false); e != nil {
			return false, e
		}
		if e = ops.Div2(newA); e != nil {
			return false, e
		}
		if e := m.refinableSignAssertion(2734, snB, s2); e != nil {
			return false, e
		}
		if e = bqm.Swap(a2, newB); e != nil {
			return false, e
		}
		if e = ops.AddSub(b, a2, newB, false); e != nil {
			return false, e
		}
		if e = ops.Div2(newB); e != nil {
			return false, e
		}
	}
}
func (m *UPolynomialManager) RefineCore(v UPolynomialView, sa int, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational) (bool, error) {
	return m.refineCoreWithEvaluationReceipt(v, sa, bqm, a, b, nil, nil)
}
func (m *UPolynomialManager) refineCoreWithEvaluationReceipt(v UPolynomialView, sa int, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational, receipt *upolyBinaryEvaluationInvocationReceipt, temporaries *algebraicRefinementScope) (interval bool, cause error) {
	if e := m.isolationBinary(bqm); e != nil {
		return false, e
	}
	if e := m.refinementSourceSignAssert(v, a, sa, 2758, "sign_a == eval_sign_at(sz, p, a)", receipt); e != nil {
		return false, e
	}
	if sa < math.MinInt32 || sa > math.MaxInt32 || sa == math.MinInt32 {
		return false, arena.ErrBounds
	}
	sb := -sa
	if e := m.refinementSourceSignAssert(v, b, sb, 2761, "sign_b == eval_sign_at(sz, p, b)", receipt); e != nil {
		return false, e
	}
	if e := refinementSourceAssert(sa == -sb, 2762, "sign_a == -sign_b"); e != nil {
		return false, e
	}
	if e := refinementSourceAssert(sa != 0 && sb != 0, 2763, "sign_a != 0 && sign_b != 0"); e != nil {
		return false, e
	}
	var local unumeral.BinaryRational
	mid := &local
	if temporaries == nil {
		defer bqm.Del(mid)
	} else {
		var e error
		mid, e = temporaries.construct(boundRefinementMid)
		if e != nil {
			return false, e
		}
		defer func() { cause = errors.Join(cause, temporaries.sourceFinish(boundRefinementMid)) }()
	}
	var arithmetic error
	if temporaries == nil {
		arithmetic = bqm.Add(a, b, mid)
	} else {
		arithmetic = bqm.WithAllocationContext(temporaries.context()).AddSub(a, b, mid, false)
	}
	if e := arithmetic; e != nil {
		return false, e
	}
	if temporaries == nil {
		arithmetic = bqm.Div2(mid)
	} else {
		arithmetic = bqm.WithAllocationContext(temporaries.context()).Div2(mid)
	}
	if e := arithmetic; e != nil {
		return false, e
	}
	sm, e := m.evalSignBinaryWithNegScopeAndReceipt(v, mid, nil, receipt)
	if e != nil {
		return false, e
	}
	if sm == 0 {
		if e = bqm.Swap(mid, a); e != nil {
			return false, e
		}
		return false, nil
	}
	if sm == sa {
		if e = bqm.Swap(mid, a); e != nil {
			return false, e
		}
		return true, nil
	}
	if e := refinementSourceAssert(sm == sb, 2776, "sign_mid == sign_b"); e != nil {
		return false, e
	}
	if e = bqm.Swap(mid, b); e != nil {
		return false, e
	}
	return true, nil
}
func (m *UPolynomialManager) Refine(v UPolynomialView, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational) (bool, error) {
	if e := m.isolationBinary(bqm); e != nil {
		return false, e
	}
	sa, e := m.EvalSignBinary(v, a)
	if e != nil {
		return false, e
	}
	if e := refinementSourceAssert(sa != 0, 2784, "sign_a != 0"); e != nil {
		return false, e
	}
	return m.RefineCore(v, sa, bqm, a, b)
}
func (m *UPolynomialManager) RefineCoreToPrecision(v UPolynomialView, sa int, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational, k uint32) (bool, error) {
	return m.refineCoreToPrecisionWithEvaluationReceipt(v, sa, bqm, a, b, k, nil, nil)
}
func (m *UPolynomialManager) refineCoreToPrecisionWithEvaluationReceipt(v UPolynomialView, sa int, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational, k uint32, receipt *upolyBinaryEvaluationInvocationReceipt, temporaries *algebraicRefinementScope) (interval bool, cause error) {
	if e := m.isolationBinary(bqm); e != nil {
		return false, e
	}
	if e := refinementSourceAssert(sa != 0, 2794, "sign_a != 0"); e != nil {
		return false, e
	}
	if e := m.refinementSourceSignAssert(v, a, sa, 2795, "sign_a  == eval_sign_at(sz, p, a)", receipt); e != nil {
		return false, e
	}
	if e := m.refinementSourceSignAssert(v, b, -sa, 2796, "-sign_a == eval_sign_at(sz, p, b)", receipt); e != nil {
		return false, e
	}
	var local unumeral.BinaryRational
	width := &local
	if temporaries == nil {
		defer bqm.Del(width)
	} else {
		var e error
		width, e = temporaries.construct(boundRefinementWidth)
		if e != nil {
			return false, e
		}
		defer func() { cause = errors.Join(cause, temporaries.sourceFinish(boundRefinementWidth)) }()
	}
	for {
		if e := m.checkpoint(); e != nil {
			return false, e
		}
		var arithmetic error
		if temporaries == nil {
			arithmetic = bqm.Sub(b, a, width)
		} else {
			arithmetic = bqm.WithAllocationContext(temporaries.context()).AddSub(b, a, width, true)
		}
		if e := arithmetic; e != nil {
			return false, e
		}
		var less bool
		var e error
		if temporaries == nil {
			less, e = bqm.LessOneDiv2k(width, k)
		} else {
			less, e = bqm.WithAllocationContext(temporaries.context()).LessOneDiv2k(width, k)
		}
		if e != nil {
			return false, e
		}
		if less {
			return true, nil
		}
		interval, e := m.refineCoreWithEvaluationReceipt(v, sa, bqm, a, b, receipt, temporaries)
		if e != nil {
			return false, e
		}
		if !interval {
			return false, nil
		}
	}
}
func (m *UPolynomialManager) RefineToPrecision(v UPolynomialView, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational, k uint32) (bool, error) {
	return m.refineToPrecisionWithEvaluationReceipt(v, bqm, a, b, k, nil, nil)
}
func (m *UPolynomialManager) refineToPrecisionWithEvaluationReceipt(v UPolynomialView, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational, k uint32, receipt *upolyBinaryEvaluationInvocationReceipt, temporaries *algebraicRefinementScope) (interval bool, cause error) {
	if e := m.isolationBinary(bqm); e != nil {
		return false, e
	}
	sa, e := m.evalSignBinaryWithNegScopeAndReceipt(v, a, nil, receipt)
	if e != nil {
		return false, e
	}
	if e := m.refinementSourceSignAssert(v, b, -sa, 2812, "eval_sign_at(sz, p, b) == -sign_a", receipt); e != nil {
		return false, e
	}
	if e := refinementSourceAssert(sa != 0, 2813, "sign_a != 0"); e != nil {
		return false, e
	}
	return m.refineCoreToPrecisionWithEvaluationReceipt(v, sa, bqm, a, b, k, receipt, temporaries)
}
func (m *UPolynomialManager) ConvertRationalToBinaryInterval(v UPolynomialView, qm *unumeral.RationalManager, a, b *unumeral.Rational, bqm *unumeral.BinaryRationalManager, c, d *unumeral.BinaryRational) (bool, error) {
	return m.convertRationalToBinaryIntervalWithEntryReceipt(v, qm, a, b, bqm, c, d, nil)
}
func (m *UPolynomialManager) convertRationalToBinaryIntervalWithEntryReceipt(v UPolynomialView, qm *unumeral.RationalManager, a, b *unumeral.Rational, bqm *unumeral.BinaryRationalManager, c, d *unumeral.BinaryRational, receipt *upolyConvertEntryReceipt) (result bool, cause error) {
	if e := m.isolationBinary(bqm); e != nil {
		return false, e
	}
	if qm == nil || qm.Integers() != m.config.Integer {
		return false, ErrUPolynomialOwner
	}
	var first, second *upolyRationalEvaluationInvocationReceipt
	if receipt != nil {
		*receipt = upolyConvertEntryReceipt{owner: m}
		first, second = &receipt.signs[0], &receipt.signs[1]
	}
	sa, e := m.evalSignRationalWithReceipt(v, a, first)
	if e != nil {
		return false, e
	}
	sb, e := m.evalSignRationalWithReceipt(v, b, second)
	if e != nil {
		return false, e
	}
	if e := m.convertIntervalEntryAssertions(sa, sb); e != nil {
		return false, e
	}
	foundD := false
	if e := m.traceConvertEntry(bqm, a, b, sa, sb, receipt); e != nil {
		return false, e
	}
	q, e := m.beginIntervalBounds(bqm, receipt)
	if e != nil {
		return false, e
	}
	defer func() { cause = q.finish(cause) }()
	lower, upper := &q.values[0], &q.values[1]
	exact, e := bqm.FromRationalWithReceipt(qm, a, lower, &q.conversion)
	if e != nil {
		return false, e
	}
	if exact {
		if e = q.trace(2830, nil); e != nil {
			return false, e
		}
		if e = bqm.Swap(c, lower); e != nil {
			return false, e
		}
		if e := m.convertBoundAssertion(2833, bqm, c, qm, a); e != nil {
			return false, e
		}
	} else {
		if e = bqm.CopyWithAllocationReceipt(upper, lower, &q.copy); e != nil {
			return false, e
		}
		if e = bqm.Mul2(upper); e != nil {
			return false, e
		}
		neg, e := m.config.Integer.IsNeg(&(*a.BorrowNumerator()))
		if e != nil {
			return false, e
		}
		if neg {
			*lower, *upper = *upper, *lower
		}
		if e = q.trace(2841, a); e != nil {
			return false, e
		}
		if e := m.convertBoundAssertion(2844, bqm, lower, qm, a); e != nil {
			return false, e
		}
		if e := m.convertBoundAssertion(2845, bqm, upper, qm, a); e != nil {
			return false, e
		}

		for {
			ge, e := bqm.GreaterEqualRational(upper, qm, b)
			if e != nil {
				return false, e
			}
			if !ge {
				break
			}
			if e = bqm.RefineUpperWithReceipt(qm, a, lower, upper, &q.refinement); e != nil {
				return false, e
			}
		}
		if e := m.convertBoundAssertion(2849, bqm, upper, qm, b); e != nil {
			return false, e
		}

		for {
			sign, e := q.eval(v, upper)
			if e != nil {
				return false, e
			}
			if sign == 0 {
				if e = bqm.Swap(c, upper); e != nil {
					return false, e
				}
				if e = unumeral.SourceIntegerDel(m.config.Integer, lower.BorrowNumerator()); e != nil {
					return false, e
				}
				if e = unumeral.SourceIntegerDel(m.config.Integer, upper.BorrowNumerator()); e != nil {
					return false, e
				}
				return false, nil
			}
			if sign == sa {
				if e = bqm.Swap(c, upper); e != nil {
					return false, e
				}
				break
			}
			if e := m.convertUpperSignAssertion(sign, sb); e != nil {
				return false, e
			}
			if !foundD {
				foundD = true
				if e = bqm.CopyWithAllocationReceipt(d, upper, &q.copy); e != nil {
					return false, e
				}
			}
			if e = bqm.RefineUpperWithReceipt(qm, a, lower, upper, &q.refinement); e != nil {
				return false, e
			}
		}
	}
	if !foundD {
		exact, e = bqm.FromRationalWithReceipt(qm, b, lower, &q.conversion)
		if e != nil {
			return false, e
		}
		if exact {
			if e = bqm.Swap(d, lower); e != nil {
				return false, e
			}
			if e := m.convertBoundAssertion(2879, bqm, d, qm, b); e != nil {
				return false, e
			}
		} else {
			if e = bqm.CopyWithAllocationReceipt(upper, lower, &q.copy); e != nil {
				return false, e
			}
			if e = bqm.Mul2(upper); e != nil {
				return false, e
			}
			neg, e := m.config.Integer.IsNeg(&(*b.BorrowNumerator()))
			if e != nil {
				return false, e
			}
			if neg {
				*lower, *upper = *upper, *lower
			}
			if e := m.convertBoundAssertion(2887, bqm, upper, qm, b); e != nil {
				return false, e
			}
			if e := m.convertBoundAssertion(2888, bqm, lower, qm, b); e != nil {
				return false, e
			}

			for {
				le, e := bqm.LessEqual(lower, c)
				if e != nil {
					return false, e
				}
				if !le {
					break
				}
				if e = bqm.RefineLowerWithReceipt(qm, b, lower, upper, &q.refinement); e != nil {
					return false, e
				}
			}
			if e := m.convertBinaryOrderAssertion(2892, bqm, c, lower); e != nil {
				return false, e
			}
			if e := m.convertBinaryOrderAssertion(2893, bqm, lower, upper); e != nil {
				return false, e
			}
			if e := m.convertBoundAssertion(2894, bqm, lower, qm, b); e != nil {
				return false, e
			}

			for {
				sign, e := q.eval(v, lower)
				if e != nil {
					return false, e
				}
				if sign == 0 {
					if e = bqm.Swap(c, lower); e != nil {
						return false, e
					}
					if e = unumeral.SourceIntegerDel(m.config.Integer, lower.BorrowNumerator()); e != nil {
						return false, e
					}
					if e = unumeral.SourceIntegerDel(m.config.Integer, upper.BorrowNumerator()); e != nil {
						return false, e
					}
					return false, nil
				}
				if sign == sb {
					if e = bqm.Swap(d, lower); e != nil {
						return false, e
					}
					break
				}
				if e = bqm.RefineLowerWithReceipt(qm, b, lower, upper, &q.refinement); e != nil {
					return false, e
				}
			}
		}
	}
	if e := m.convertBinaryOrderAssertion(2912, bqm, c, d); e != nil {
		return false, e
	}
	if e := m.convertFinalSignAssertionWithReceipt(v, c, d, &q.evaluation); e != nil {
		return false, e
	}
	if e := m.convertBoundAssertion(2914, bqm, c, qm, a); e != nil {
		return false, e
	}
	if e := m.convertBoundAssertion(2915, bqm, d, qm, b); e != nil {
		return false, e
	}

	return true, nil
}
func (m *UPolynomialManager) NormalizeIntervalCore(v UPolynomialView, sa int, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational) (bool, error) {
	if e := m.isolationBinary(bqm); e != nil {
		return false, e
	}
	neg, e := bqm.IsNeg(a)
	if e != nil {
		return false, e
	}
	if !neg {
		return true, nil
	}
	pos, e := bqm.IsPos(b)
	if e != nil {
		return false, e
	}
	if !pos {
		return true, nil
	}
	if sa == math.MinInt32 {
		sa, e = m.EvalSignBinary(v, a)
		if e != nil {
			return false, e
		}
	} else {
		if e := m.refinementSourceSignAssert(v, a, sa, 2926, "sign_a == eval_sign_at(sz, p, a)", nil); e != nil {
			return false, e
		}
	}
	sb := -sa
	if e := m.refinementSourceSignAssert(v, b, sb, 2930, "sign_b == eval_sign_at(sz, p, b)", nil); e != nil {
		return false, e
	}
	if e := m.normalizeIntervalNonzeroAssertion(sa, sb); e != nil {
		return false, e
	}

	zero, e := m.HasZeroRoots(v)
	if e != nil {
		return false, e
	}
	if zero {
		return false, nil
	}
	sz, e := m.EvalSignZero(v)
	if e != nil {
		return false, e
	}
	if sa == sz {
		e = bqm.Reset(a)
	} else {
		e = bqm.Reset(b)
	}
	if e != nil {
		return false, e
	}
	if e := m.normalizeIntervalFinalSignAssertion(v, a, b); e != nil {
		return false, e
	}

	return true, nil
}
func (m *UPolynomialManager) NormalizeInterval(v UPolynomialView, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational) (bool, error) {
	return m.NormalizeIntervalCore(v, math.MinInt32, bqm, a, b)
}
