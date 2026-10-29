package numeral

// Source: src/util/mpf.cpp:335-374; src/util/mpf.h:86.

import (
	"errors"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func (m *ArbitraryFloatManager) SetFormat(out *ArbitraryFloat, ebits, sbits uint32, mode ArbitraryFloatRoundingMode, x *ArbitraryFloat) error {
	return m.SetFormatWithReceipt(out, ebits, sbits, mode, x, nil)
}
func (m *ArbitraryFloatManager) SetFormatWithReceipt(out *ArbitraryFloat, ebits, sbits uint32, mode ArbitraryFloatRoundingMode, x *ArbitraryFloat, r *ArbitraryFloatFormattedSetReceipt) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	if x == nil {
		return umemory.ErrRuntimeOwner
	}
	s := &m.formattedSet
	if s.active || s.executing || s.finishing || s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	invocation := s.invocation + 1
	*s = arbitraryFloatFormattedSetScope{manager: m, invocation: invocation, active: true, executing: true, receipt: r}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = errors.Join(cause, m.finishFormattedSetSource())
		s.recordHistory()
	}()
	nan, e := m.IsNaN(x)
	if e != nil {
		return e
	}
	if nan {
		return m.mkNaNWithSourceContext(ebits, sbits, out, &s.copy, s.allocationContext())
	}
	inf, e := m.IsInf(x)
	if e != nil {
		return e
	}
	if inf {
		return m.MkInf(ebits, sbits, x.Sign(), out)
	}
	zero, e := m.IsZero(x)
	if e != nil {
		return e
	}
	if zero {
		return m.MkZero(ebits, sbits, x.Sign(), out)
	}
	c := s.allocationContext()
	same := x.EBits() == ebits && x.SBits() == sbits
	if e = m.copyWithValueReceiptAndContext(out, x, &s.copy, c); e != nil {
		return e
	}
	if same {
		return nil
	}
	if e = m.unpackRationalNumeral(out, true, c); e != nil {
		return e
	}
	out.assignEbits(ebits)
	out.assignSbits(sbits)
	// Source subtraction is unsigned32, followed by the target signed32
	// conversion. Read x.sbits here, including the literal self-alias behavior.
	ds := int32(sbits - x.SBits() + 3)
	if ds > 0 {
		if e = m.integer.mul2kCoreWithAllocationContext(&out.significand, uint32(ds), c); e != nil {
			return e
		}
		return m.roundWithFormattedSet(mode, out, s)
	}
	if ds < 0 {
		sticky := false
		for ds < 0 {
			even, e := m.integer.IsEven(&out.significand)
			if e != nil {
				return e
			}
			sticky = sticky || !even
			if e = m.integer.MachineDiv2kInPlace(&out.significand, 1); e != nil {
				return e
			}
			ds++
		}
		if sticky {
			even, e := m.integer.IsEven(&out.significand)
			if e != nil {
				return e
			}
			if even {
				one := NewInt(1)
				if e = m.integer.addSubWithAllocationContext(&out.significand, &one, &out.significand, false, c); e != nil {
					return e
				}
			}
		}
		return m.roundWithFormattedSet(mode, out, s)
	}
	return nil
}
