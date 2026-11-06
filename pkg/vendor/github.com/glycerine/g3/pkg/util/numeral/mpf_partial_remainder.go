package numeral

// Source: src/util/mpf.cpp:1272-1436; src/util/mpf.h:232.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"math"
)

// PartialRemainder operates on normalized, unpacked inputs, as the source
// helper does. It updates x; y remains borrowed through the final publication.
func (m *ArbitraryFloatManager) PartialRemainder(x, y *ArbitraryFloat, expDiff int64, partial bool) (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.partialRemainder.active {
		return ErrNumericPending
	}
	for _, v := range [...]*ArbitraryFloat{x, y} {
		if e := m.valueGuard(v); e != nil {
			return e
		}
	}
	if e := mpfSourceAssertion(expDiff >= -1 && expDiff < math.MaxInt64, 1276, "-1 <= exp_diff && exp_diff < INT64_MAX"); e != nil {
		return e
	}
	if e := mpfSourceAssertion(expDiff < int64(x.EBits()+x.SBits()) || partial, 1277, "exp_diff < ebits+sbits || partial"); e != nil {
		return e
	}
	s := &m.partialRemainder
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatPartialRemainderScope{manager: m, invocation: s.invocation + 1, active: true, executing: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishPartialRemainderSource())
		cause = JoinNumericErrors(cause, s.unbind())
	}()
	return s.run(x, y, expDiff, partial)
}
func (s *arbitraryFloatPartialRemainderScope) power(n uint32) (Int, error) {
	w, e := s.manager.powerTwoWrapper(n, false)
	if e != nil {
		return Int{}, e
	}
	return w.Capture()
}
func (s *arbitraryFloatPartialRemainderScope) sourceAssertPower(v *Int, n uint32, ge bool, line int32, expression string) error {
	enabled, e := mpfSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	p, e := s.power(n)
	if e != nil {
		return e
	}
	cmp, e := s.manager.integer.Compare(v, &p)
	if e != nil {
		return e
	}
	return mpfSourceAssertion(ge && cmp >= 0 || !ge && cmp < 0, line, expression)
}
func (s *arbitraryFloatPartialRemainderScope) shift(v *Int, n uint32) error {
	z := s.manager.integer
	if e := z.check(v); e != nil {
		return e
	}
	return z.mul2kCoreWithAllocationContext(v, n, s.context())
}
func (s *arbitraryFloatPartialRemainderScope) copyValue(dst, src *Int) error {
	z := s.manager.integer
	return z.copyFromOwnerWithAllocationContext(dst, z, src, &s.copy, s.context())
}
func (s *arbitraryFloatPartialRemainderScope) inc(v *Int) error {
	one := NewInt(1)
	return s.manager.integer.addSubWithAllocationContext(v, &one, v, false, s.context())
}
func (s *arbitraryFloatPartialRemainderScope) endNumeral(index int) error {
	if !s.live[index] {
		return nil
	}

	if _, e := SourceNumeralDel(s.manager.integer, &s.values[index]); e != nil {
		s.sourceCause = e
		return e
	}
	s.live[index] = false
	return nil
}
func (s *arbitraryFloatPartialRemainderScope) run(x, y *ArbitraryFloat, expDiff int64, partial bool) error {
	if !s.NumericBound(true) {
		return ErrNumericOwner
	}
	m, z, c := s.manager, s.manager.integer, s.context()
	eb, sb := x.EBits(), x.SBits()
	d := int64(int32(expDiff))
	n := int64(sb - 1)
	if e := m.tracePartialRemainderEntry(x, y, d, partial); e != nil {
		return e
	}
	if e := s.sourceAssertPower(&x.significand, sb, false, 1287, "m_mpz_manager.lt(x.significand, m_powers2(sbits))"); e != nil {
		return e
	}
	if e := s.sourceAssertPower(&x.significand, sb-1, true, 1288, "m_mpz_manager.ge(x.significand, m_powers2(sbits - 1))"); e != nil {
		return e
	}
	if e := s.sourceAssertPower(&y.significand, sb, false, 1289, "m_mpz_manager.lt(y.significand, m_powers2(sbits))"); e != nil {
		return e
	}
	if e := s.sourceAssertPower(&y.significand, sb-1, true, 1290, "m_mpz_manager.ge(y.significand, m_powers2(sbits - 1))"); e != nil {
		return e
	}
	// Actual source declarations: shifted numerator, large quotient, its
	// remainder, and the unused x_rem_y_sig are all constructed here.
	for i := 0; i < 4; i++ {
		s.live[i] = true
	}
	shifted, large, divRem := &s.values[0], &s.values[1], &s.values[2]
	if e := z.shiftWithAllocationContext(&x.significand, 2*sb+2, shifted, true, c); e != nil {
		return e
	}
	if e := z.machineDivRemWithAllocationContext(shifted, &y.significand, large, divRem, c); e != nil {
		return e
	}
	if e := m.tracePartialRemainderRatio(x.Sign() != y.Sign(), d, large, divRem, eb, sb); e != nil {
		return e
	}
	s.live[4], s.live[5] = true, true
	q, qr := &s.values[4], &s.values[5]
	qe := d
	chosen := qe
	if partial {
		chosen = n
	}
	qShift := (sb - 1) + (sb + 3) - uint32(chosen)
	if partial {
		if e := mpfSourceAssertion(0 < n && n < qe && qe < 2147483647, 1313, "0 < N && N < Q_exp && Q_exp < INT_MAX"); e != nil {
			return e
		}
		if e := z.shiftWithAllocationContext(large, qShift, q, false, c); e != nil {
			return e
		}
	} else {
		p, e := s.power(qShift)
		if e != nil {
			return e
		}
		if e := z.machineDivRemWithAllocationContext(large, &p, q, qr, c); e != nil {
			return e
		}
		half, e := s.power(qShift - 1)
		if e != nil {
			return e
		}
		tie, e := z.Equal(qr, &half)
		if e != nil {
			return e
		}
		more, e := z.Greater(qr, &half)
		if e != nil {
			return e
		}
		m.tracePartialRemainderTie(tie, more)
		increment := more
		if tie {
			odd, e := z.IsOdd(q)
			if e != nil {
				return e
			}
			increment = increment || odd
		}
		if increment {
			if e := s.inc(q); e != nil {
				return e
			}
		}
	}
	if e := s.shift(q, qShift); e != nil {
		return e
	}
	if e := z.MachineDiv2kInPlace(q, sb+3); e != nil {
		return e
	}
	if e := m.renormalizeWithContext(eb, sb, &qe, q, c); e != nil {
		return e
	}
	if e := m.tracePartialRemainderQ(x.Sign() != y.Sign(), qe, q, eb, sb); e != nil {
		return e
	}
	if d == -1 || partial {
		empty, e := z.IsZero(q)
		if e != nil {
			return e
		}
		if empty {
			return nil
		}
	}
	assertionsEnabled, e := mpfSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if assertionsEnabled {
		empty, e := z.IsZero(q)
		if e != nil {
			return e
		}
		if e := mpfSourceAssertion(!empty, 1339, "!m_mpz_manager.is_zero(Q_sig)"); e != nil {
			return e
		}
	}
	if e := s.sourceAssertPower(q, sb, false, 1340, "m_mpz_manager.lt(Q_sig, m_powers2(sbits))"); e != nil {
		return e
	}
	if e := s.sourceAssertPower(q, sb-1, true, 1341, "m_mpz_manager.ge(Q_sig, m_powers2(sbits - 1))"); e != nil {
		return e
	}
	s.live[6] = true
	product := &s.values[6]
	if y.exponent > 0 && qe > math.MaxInt64-y.exponent || y.exponent < 0 && qe < math.MinInt64-y.exponent {
		return arena.ErrBounds
	}
	pe := qe + y.exponent
	if e := z.mulWithAllocationContext(&y.significand, q, product, c); e != nil {
		return e
	}
	if e := m.renormalizeWithContext(eb, 2*sb-1, &pe, product, c); e != nil {
		return e
	}
	if e := m.tracePartialRemainderProduct(x.Sign(), pe, product, eb, sb); e != nil {
		return e
	}
	if e := s.sourceAssertPower(product, 2*sb-1, false, 1358, "m_mpz_manager.lt(YQ_sig, m_powers2(2*sbits-1))"); e != nil {
		return e
	}
	assertionsEnabled, e = mpfSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if assertionsEnabled {
		p, e := s.power(2*sb - 2)
		if e != nil {
			return e
		}
		ge, e := z.GreaterEqual(product, &p)
		if e != nil {
			return e
		}
		predicate := ge
		if !predicate {
			bottom, e := m.bottomExponent(eb)
			if e != nil {
				return e
			}
			predicate = pe <= bottom
		}
		if e := mpfSourceAssertion(predicate, 1359, "m_mpz_manager.ge(YQ_sig, m_powers2(2*sbits-2)) || YQ_exp <= mk_bot_exp(ebits)"); e != nil {
			return e
		}
	}
	re := x.exponent
	s.live[7] = true
	result := &s.values[7]
	delta, e := roundSignedSub(x.exponent, pe)
	if e != nil {
		return e
	}
	m.tracePartialRemainderExponentDelta(delta)
	if e := mpfSourceAssertion(delta > -2147483648 && delta <= 2147483647, 1366, "INT_MIN < exp_delta && exp_delta <= INT_MAX"); e != nil {
		return e
	}
	s.live[8], s.live[9], s.live[10] = true, true, true
	minuend, subtrahend, xLarge := &s.values[8], &s.values[9], &s.values[10]
	if e := z.shiftWithAllocationContext(&x.significand, sb-1, xLarge, true, c); e != nil {
		return e
	}
	if e := s.copyValue(minuend, xLarge); e != nil {
		return e
	}
	if e := s.copyValue(subtrahend, product); e != nil {
		return e
	}
	if e := s.sourceAssertPower(minuend, 2*sb-1, false, 1375, "m_mpz_manager.lt(minuend, m_powers2(2*sbits-1))"); e != nil {
		return e
	}
	if e := s.sourceAssertPower(minuend, 2*sb-2, true, 1376, "m_mpz_manager.ge(minuend, m_powers2(2*sbits-2))"); e != nil {
		return e
	}
	if e := s.sourceAssertPower(subtrahend, 2*sb-1, false, 1377, "m_mpz_manager.lt(subtrahend, m_powers2(2*sbits-1))"); e != nil {
		return e
	}
	if e := s.sourceAssertPower(subtrahend, 2*sb-2, true, 1378, "m_mpz_manager.ge(subtrahend, m_powers2(2*sbits-2))"); e != nil {
		return e
	}
	if delta != 0 {
		s.live[11] = true
		sticky := &s.values[11]
		if e := z.SetInt32(sticky, 0); e != nil {
			return e
		}
		if delta > int64(sb+5) {
			if e := z.Swap(sticky, subtrahend); e != nil {
				return e
			}
		} else if delta > 0 {
			p, e := s.power(uint32(delta))
			if e != nil {
				return e
			}
			if e := z.machineDivRemWithAllocationContext(subtrahend, &p, subtrahend, sticky, c); e != nil {
				return e
			}
		} else {
			if e := mpfSourceAssertion(delta < 0, 1388, "exp_delta < 0"); e != nil {
				return e
			}
			if delta == -1<<63 {
				return arena.ErrBounds
			}
			delta = -delta
			if e := s.shift(subtrahend, uint32(int32(delta))); e != nil {
				return e
			}
		}
		empty, e := z.IsZero(sticky)
		if e != nil {
			return e
		}
		if !empty {
			even, e := z.IsEven(subtrahend)
			if e != nil {
				return e
			}
			if even {
				if e := s.inc(subtrahend); e != nil {
					return e
				}
			}
		}
		if e := m.tracePartialRemainderInt(1394, "aligned subtrahend=", subtrahend); e != nil {
			return e
		}
		if e := s.endNumeral(11); e != nil {
			return e
		}
	}
	if e := z.addSubWithAllocationContext(minuend, subtrahend, result, true, c); e != nil {
		return e
	}
	if e := m.tracePartialRemainderInt(1398, "X_YQ_sig'=", result); e != nil {
		return e
	}
	neg, e := z.IsNeg(result)
	if e != nil {
		return e
	}
	if neg {
		if e := z.Neg(result); e != nil {
			return e
		}
	}
	sign := x.Sign() != neg
	empty, e := z.IsZero(result)
	if e != nil {
		return e
	}
	if empty {
		if e := m.MkZero(eb, sb, x.Sign(), x); e != nil {
			return e
		}
		return m.tracePartialRemainderObject(x)
	}
	if e := m.renormalizeWithContext(eb, 2*sb-1, &re, result, c); e != nil {
		return e
	}
	if e := m.tracePartialRemainderDifference(minuend, subtrahend, sign, re, result, eb, sb); e != nil {
		return e
	}
	if e := s.sourceAssertPower(result, 2*sb-1, false, 1418, "m_mpz_manager.lt(X_YQ_sig, m_powers2(2*sbits-1))"); e != nil {
		return e
	}
	s.live[12] = true
	rnd := &s.values[12]
	p, e := s.power(sb - 1)
	if e != nil {
		return e
	}
	if e := z.machineDivRemWithAllocationContext(result, &p, result, rnd, c); e != nil {
		return e
	}
	if e := m.tracePartialRemainderInt(1421, "final sticky=", rnd); e != nil {
		return e
	}
	// Preserve the source constant, independently of sbits.
	half := NewInt(32)
	tie, e := z.Equal(rnd, &half)
	if e != nil {
		return e
	}
	if tie {
		odd, e := z.IsOdd(result)
		if e != nil {
			return e
		}
		if odd {
			if e := s.inc(result); e != nil {
				return e
			}
		}
	} else {
		more, e := z.Greater(rnd, &half)
		if e != nil {
			return e
		}
		if more {
			if e := s.inc(result); e != nil {
				return e
			}
		}
	}
	// mpz-significand SET publishes the exponent only after successful COPY.
	x.format = arbitraryFloatFormat(eb, sb, sign)
	if e := s.copyValue(&x.significand, result); e != nil {
		return e
	}
	x.exponent = re
	if e := s.endNumeral(12); e != nil {
		return e
	}
	return m.tracePartialRemainderObject(x)
}
