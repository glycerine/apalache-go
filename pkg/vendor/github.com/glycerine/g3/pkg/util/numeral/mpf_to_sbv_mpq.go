package numeral

// Source: src/util/mpf.cpp:1183-1226.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// ToSBVMPQ rounds to an integer rational using the manager's own numeral
// namespace. The output remains caller-owned through the MPF manager lifetime.
func (m *ArbitraryFloatManager) ToSBVMPQ(mode ArbitraryFloatRoundingMode, x *ArbitraryFloat, out *Rational) (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.sbv.active {
		return ErrNumericPending
	}
	if e := m.valueGuard(x); e != nil {
		return e
	}
	if e := m.rational.checkSetter(out); e != nil {
		return e
	}
	assertionsEnabled, e := mpfSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if assertionsEnabled {
		nan, e := m.IsNaN(x)
		if e != nil {
			return e
		}
		inf := false
		if !nan {
			inf, e = m.IsInf(x)
			if e != nil {
				return e
			}
		}
		if e := mpfSourceAssertion(!nan && !inf, 1184, "!is_nan(x) && !is_inf(x)"); e != nil {
			return e
		}
	}
	s := &m.sbv
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatSBVScope{manager: m, invocation: s.invocation + 1, active: true, executing: true, aLive: true, zLive: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishSBVSource())
		cause = JoinNumericErrors(cause, s.unbind())
	}()
	if e := s.run(mode, x, out); e != nil {
		return e
	}
	return m.traceSBVResult(out)
}
func (s *arbitraryFloatSBVScope) run(mode ArbitraryFloatRoundingMode, x *ArbitraryFloat, out *Rational) error {
	if !s.NumericBound(true) {
		return ErrNumericOwner
	}
	m, z, c := s.manager, s.manager.integer, s.context()
	if e := m.copyWithValueReceiptAndContext(&s.a, x, &s.copyA, c); e != nil {
		return e
	}
	if e := m.unpackRationalNumeral(&s.a, true, c); e != nil {
		return e
	}
	if e := mpfSourceAssertion(s.a.exponent < math.MaxInt32, 1192, "t.exponent() < INT_MAX"); e != nil {
		return e
	}
	if e := z.copyFromOwnerWithAllocationContext(&s.z, z, &s.a.significand, &s.copyZ, c); e != nil {
		return e
	}
	// Here sbits() returns unsigned32, promoted with the int64 exponent.
	// Its subtraction and the following +1 remain separate signed expressions.
	exponent, e := roundSignedSub(s.a.exponent, int64(s.a.SBits()))
	if e != nil {
		return e
	}
	if exponent == math.MaxInt64 {
		return arena.ErrBounds
	}
	exponent++
	if exponent < 0 {
		last, e := z.IsOdd(&s.z)
		if e != nil {
			return e
		}
		round, sticky := false, false
		for ; exponent != 0; exponent++ {
			if e := z.MachineDiv2kInPlace(&s.z, 1); e != nil {
				return e
			}
			sticky = sticky || round
			round = last
			last, e = z.IsOdd(&s.z)
			if e != nil {
				return e
			}
		}
		increment := false
		switch mode {
		case ArbitraryFloatNearestEven:
			increment = round && (last || sticky)
		case ArbitraryFloatNearestAway:
			increment = round
		case ArbitraryFloatTowardPositive:
			increment = !x.Sign() && (round || sticky)
		case ArbitraryFloatTowardNegative:
			increment = x.Sign() && (round || sticky)
		case ArbitraryFloatTowardZero:
		default:
			if e := diagnostic.BorrowProcessDebugContext().UnreachableSource("src/util/mpf.cpp", 1211); e != nil {
				return e
			}
		}
		if increment {
			one := NewInt(1)
			if e := z.addSubWithAllocationContext(&s.z, &one, &s.z, false, c); e != nil {
				return e
			}
		}
		if e := m.traceSBVDecision(x, &s.z, last, round, sticky, increment); e != nil {
			return e
		}
	} else {
		if e := z.check(&s.z); e != nil {
			return e
		}
		if e := z.mul2kCoreWithAllocationContext(&s.z, uint32(exponent), c); e != nil {
			return e
		}
	}
	if e := m.rational.setIntegerFromOwnerWithReceiptAndAllocationContext(out, z, &s.z, &s.outputCopy, c); e != nil {
		return e
	}
	if x.Sign() {
		return m.rational.negWithSourceContext(out, c)
	}
	return nil
}
