package numeral

// Source: src/util/mpf.cpp:233-300; src/util/mpq.cpp:312-315.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// SetExponentRational follows the literal normalization of significand*2^exponent.
func (m *ArbitraryFloatManager) SetExponentRational(out *ArbitraryFloat, ebits, sbits uint32, mode ArbitraryFloatRoundingMode, exponent *Int, significand *Rational) error {
	return m.SetExponentRationalWithReceipt(out, ebits, sbits, mode, exponent, significand, nil)
}
func (m *ArbitraryFloatManager) SetExponentRationalWithReceipt(out *ArbitraryFloat, ebits, sbits uint32, mode ArbitraryFloatRoundingMode, exponent *Int, significand *Rational, r *ArbitraryFloatRationalSetReceipt) error {
	return m.setExponentRationalWithPlainCaller(out, ebits, sbits, mode, exponent, significand, r, nil)
}
func (m *ArbitraryFloatManager) setExponentRationalWithPlainCaller(out *ArbitraryFloat, ebits, sbits uint32, mode ArbitraryFloatRoundingMode, exponent *Int, significand *Rational, r *ArbitraryFloatRationalSetReceipt, plain *arbitraryFloatPlainRationalSetScope) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	if plain != nil && (!plain.matched(m) || !plain.executing || plain.finishing || !plain.expLive || exponent != &plain.exp) {
		return umemory.ErrRuntimeOwner
	}
	if significand == nil {
		return umemory.ErrRuntimeOwner
	}
	// is_neg/is_zero read only the actual numerator. The zero branch never
	// reads exponent or denominator, and constructs no lexical NUMs.
	if e := m.integer.check(&significand.Num); e != nil {
		return e
	}
	s := &m.rationalSet
	if s.executing || s.finishing || m.integer.Memory.FailureTransportActive() {
		return ErrNumericPending
	}
	if significand.Num.Val != 0 && (s.active || s.invocation == ^uint64(0)) {
		return ErrNumericPending
	}
	if e := m.traceSetExponentRationalEntry(significand, exponent); e != nil {
		return e
	}
	out.assignEbits(ebits)
	out.assignSbits(sbits)
	out.assignSign(significand.Num.Val < 0)
	if significand.Num.Val == 0 {
		if e := m.MkZero(ebits, sbits, out.Sign(), out); e != nil {
			return e
		}
		return m.traceSetOrdinaryResult(out, 300)
	}
	invocation := s.invocation + 1
	*s = arbitraryFloatRationalSetScope{manager: m, invocation: invocation, active: true, executing: true, receipt: r}
	if plain != nil {
		s.plainParent = plain
		s.plainInvocation = plain.invocation
		plain.childFrame = s
		plain.childInvocation = s.invocation
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		s.recordHistory()
		cause = errors.Join(cause, m.finishRationalSetSource())
	}()
	s.sig = NewRational()
	s.sigLive = true
	s.exp = Int{}
	s.expLive = true
	c := s.allocationContext()
	if e := m.rational.copyWithAllocationContext(&s.sig, significand, c); e != nil {
		return e
	}
	if e := m.rational.absWithAllocationContext(&s.sig, c); e != nil {
		return e
	}
	if e := m.integer.copyFromOwnerWithAllocationContext(&s.exp, m.integer, exponent, nil, c); e != nil {
		return e
	}
	one, two := NewRational(), NewRational()
	one.Num = NewInt(1)
	two.Num = NewInt(2)
	lt, e := m.rational.compareWithAllocationContext(&s.sig, &one, c)
	if e != nil {
		return e
	}
	branch := uint8(0)
	if lt < 0 {
		branch = 1
		if e = m.rational.invInPlaceWithAllocationContext(&s.sig, c); e != nil {
			return e
		}
	} else {
		ge, e := m.rational.compareWithAllocationContext(&s.sig, &two, c)
		if e != nil {
			return e
		}
		if ge >= 0 {
			branch = 2
		}
	}
	if branch != 0 {
		pp, e := m.rational.prevPowerOfTwoWithRationalSetContext(c)
		if e != nil {
			return e
		}
		if branch == 1 {
			ok, power, e := m.rational.IsPowerOfTwo(&s.sig)
			if e != nil {
				return e
			}
			if !ok || power != pp {
				pp++
			}
		}
		s.p2 = Int{}
		s.p2Live = true
		base := NewInt(2)
		if e = m.integer.powerWithAllocationContext(&base, pp, &s.p2, nil, c); e != nil {
			return e
		}
		if e = m.rational.divIntWithAllocationContext(&s.sig, &s.p2, &s.sig, c); e != nil {
			return e
		}
		adjustment := NewInt(int32(pp))
		if e = m.integer.addSubWithAllocationContext(&s.exp, &adjustment, &s.exp, branch == 1, c); e != nil {
			return e
		}
		if branch == 1 {
			if e = m.rational.invInPlaceWithAllocationContext(&s.sig, c); e != nil {
				return e
			}
		}
		if _, e = sourceNumeralDel(m.integer, &s.p2); e != nil {
			s.sourceCause = e
			return e
		}
		s.p2Live = false
	}
	if e := m.traceSetExponentRationalNormalized(&s.sig, &s.exp); e != nil {
		return e
	}
	if modularSourceDebug {
		if e := m.assertRationalSetNormalized(&s.sig, c); e != nil {
			return e
		}
	}

	s.p = Int{}
	s.pLive = true
	s.t = NewRational()
	s.tLive = true
	s.sq = NewRational()
	s.sqLive = true
	base := NewInt(2)
	if e = m.integer.powerWithAllocationContext(&base, sbits+3-1, &s.p, nil, c); e != nil {
		return e
	}
	if e = m.rational.mulIntWithAllocationContext(&s.p, &s.sig, &s.t, c); e != nil {
		return e
	}
	if e = m.rational.roundToIntWithAllocationContext(&s.t, &out.significand, false, c); e != nil {
		return e
	}
	if e = m.rational.setIntegerWithAllocationContext(&s.sq, &out.significand, c); e != nil {
		return e
	}
	if e = m.rational.divIntWithAllocationContext(&s.sq, &s.p, &s.t, c); e != nil {
		return e
	}
	if e = m.rational.addSubWithAllocationContext(&s.sig, &s.t, &s.sig, true, c); e != nil {
		return e
	}
	if s.sig.Num.Val != 0 {
		even, e := m.integer.IsEven(&out.significand)
		if e != nil {
			return e
		}
		if even {
			v := NewInt(1)
			if e = m.integer.addSubWithAllocationContext(&out.significand, &v, &out.significand, false, c); e != nil {
				return e
			}
		}
	}
	if e := m.traceSetExponentRationalFields(out); e != nil {
		return e
	}
	if s.exp.IsSmall() {
		v, e := m.integer.Int64(&s.exp)
		if e != nil {
			return e
		}
		out.exponent = v
		if e := m.roundWithParentContext(mode, out, &s.roundChild, s.allocationContext()); e != nil {
			return e
		}
	} else if e := m.MkInf(ebits, sbits, out.Sign(), out); e != nil {
		return e
	}
	return m.traceSetOrdinaryResult(out, 300)
}

// Source: src/util/mpf.cpp:274. This helper retains the actual assertion
// operands and short circuit; it adds no source construction or acquisition.
func (m *ArbitraryFloatManager) assertRationalSetNormalized(sig *Rational, c integerAllocationInvocationContext) error {
	if !modularSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	one, two := NewRational(), NewRational()
	one.Num, two.Num = NewInt(1), NewInt(2)
	low, e := m.rational.compareWithAllocationContext(&one, sig, c)
	if e != nil {
		return e
	}
	if low > 0 {
		return diagnostic.DebugAssertionFailure(d, "src/util/mpf.cpp", 274, "(m_mpq_manager.le(1, sig) && m_mpq_manager.lt(sig, 2))")
	}
	high, e := m.rational.compareWithAllocationContext(sig, &two, c)
	if e != nil {
		return e
	}
	if high >= 0 {
		return diagnostic.DebugAssertionFailure(d, "src/util/mpf.cpp", 274, "(m_mpq_manager.le(1, sig) && m_mpq_manager.lt(sig, 2))")
	}
	return nil
}
