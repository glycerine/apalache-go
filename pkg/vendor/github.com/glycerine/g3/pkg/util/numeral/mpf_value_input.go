package numeral

// Source: src/util/mpf.cpp:69-106,114-181,303-316; src/util/mpf.h:76,78-79.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// SetComponentsUint64 is the scalar-significand overload used by the FPA API.
// The source publishes format and sign before the integer SET, then exponent.
func (m *ArbitraryFloatManager) SetComponentsUint64(out *ArbitraryFloat, ebits, sbits uint32, sign bool, exponent int64, significand uint64) error {
	return m.SetComponentsUint64WithReceipt(out, ebits, sbits, sign, exponent, significand, nil)
}

func (m *ArbitraryFloatManager) SetComponentsUint64WithReceipt(out *ArbitraryFloat, ebits, sbits uint32, sign bool, exponent int64, significand uint64, receipt *ArbitraryFloatValueInputReceipt) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	context, e := m.beginValueInput(receipt)
	if e != nil {
		return e
	}
	defer func() { m.finishValueInput(cause) }()
	out.assignEbits(ebits)
	out.assignSbits(sbits)
	out.assignSign(sign)
	m.valueExecuting = true
	if e := m.integer.setUint64WithAllocationContext(&out.significand, significand, context); e != nil {
		m.valueExecuting = false
		return e
	}
	m.valueExecuting = false
	out.exponent = exponent
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled {
			if sbits == 0 {
				return arena.ErrBounds
			}
			power, e := m.powerTwoWrapper(sbits-1, false)
			if e != nil {
				return e
			}
			bound, e := power.Capture()
			if e != nil {
				return e
			}
			less, e := m.integer.Less(&out.significand, &bound)
			if e != nil {
				return e
			}
			if !less {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/mpf.cpp", 312, "m_mpz_manager.lt(o.significand, m_powers2(sbits-1))"); e != nil {
					return e
				}
			}
		}
		enabled, e = diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled {
			top, e := m.topExponent(ebits)
			if e != nil {
				return e
			}
			if exponent > top {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/mpf.cpp", 313, "o.exponent <= mk_top_exp(ebits)"); e != nil {
					return e
				}
			}
		}
		enabled, e = diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled {
			bottom, e := m.bottomExponent(ebits)
			if e != nil {
				return e
			}
			if exponent < bottom {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/mpf.cpp", 314, "o.exponent >= mk_bot_exp(ebits)"); e != nil {
					return e
				}
			}
		}
	}
	return nil
}

// SetInt32 is the literal machine-int overload. In particular it normalizes the
// uint32 word before removing the implicit leading bit; no rounding mode is
// invented for the source-disabled INT_MAX-to-single assertion.
func (m *ArbitraryFloatManager) SetInt32(out *ArbitraryFloat, ebits, sbits uint32, value int32) error {
	return m.SetInt32WithReceipt(out, ebits, sbits, value, nil)
}

func (m *ArbitraryFloatManager) SetInt32WithReceipt(out *ArbitraryFloat, ebits, sbits uint32, value int32, receipt *ArbitraryFloatValueInputReceipt) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	context, e := m.beginValueInput(receipt)
	if e != nil {
		return e
	}
	defer func() { m.finishValueInput(cause) }()
	out.assignSign(false)
	out.assignEbits(ebits)
	out.assignSbits(sbits)
	m.traceFloatDebugSigned("set", "set: value = ", int64(value), 76)
	if value == 0 {
		if e := m.MkZero(ebits, sbits, false, out); e != nil {
			return e
		}
		return m.traceSetInt32Result(out)
	}
	uval := uint32(value)
	if value < 0 {
		out.assignSign(true)
		if value == (-1 << 31) {
			uval = 0x80000000
		} else {
			uval = uint32(-value)
		}
	}
	out.exponent = 31
	for uval&0x80000000 == 0 {
		uval <<= 1
		out.exponent--
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e := m.integer.setUint32WithAllocationContext(&out.significand, uval&0x7fffffff, context); e != nil {
		return e
	}
	if sbits > 31 {
		if e := m.integer.mul2kCoreWithAllocationContext(&out.significand, sbits-32, context); e != nil {
			return e
		}
	} else if e := m.integer.MachineDiv2kInPlace(&out.significand, 32-sbits); e != nil {
		return e
	}
	m.valueExecuting = false
	return m.traceSetInt32Result(out)
}

// SetDouble copies the native IEEE bits, publishes source fields, then clamps
// the exponent and shifts the stored fraction exactly as the source overload.
func (m *ArbitraryFloatManager) SetDouble(out *ArbitraryFloat, ebits, sbits uint32, value float64) error {
	return m.SetDoubleWithReceipt(out, ebits, sbits, value, nil)
}

func (m *ArbitraryFloatManager) SetDoubleWithReceipt(out *ArbitraryFloat, ebits, sbits uint32, value float64, receipt *ArbitraryFloatValueInputReceipt) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	context, e := m.beginValueInput(receipt)
	if e != nil {
		return e
	}
	defer func() { m.finishValueInput(cause) }()
	raw := math.Float64bits(value)
	sign := raw>>63 != 0
	exp := int64((raw&0x7ff0000000000000)>>52) - 1023
	sig := raw & 0x000fffffffffffff
	m.traceSetNativeInput(value, 64, raw, sig, sign, exp, 124)
	if e := mpfSourceAssertion(-1023 <= exp && exp <= 1024, 127, "-1023 <= e && e <= +1024"); e != nil {
		return e
	}
	out.assignEbits(ebits)
	out.assignSbits(sbits)
	out.assignSign(sign)
	if e := m.assignNativeInputExponent(out, ebits, exp); e != nil {
		return e
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e := m.integer.setUint64WithAllocationContext(&out.significand, sig, context); e != nil {
		return e
	}
	if sbits < 53 {
		if e := m.integer.MachineDiv2kInPlace(&out.significand, 53-sbits); e != nil {
			return e
		}
	} else if sbits > 53 {
		if e := m.integer.mul2kCoreWithAllocationContext(&out.significand, sbits-53, context); e != nil {
			return e
		}
	}
	m.valueExecuting = false
	return m.traceSetOrdinaryResult(out, 147)
}

func (m *ArbitraryFloatManager) SetFloat(out *ArbitraryFloat, ebits, sbits uint32, value float32) error {
	return m.SetFloatWithReceipt(out, ebits, sbits, value, nil)
}

func (m *ArbitraryFloatManager) SetFloatWithReceipt(out *ArbitraryFloat, ebits, sbits uint32, value float32, receipt *ArbitraryFloatValueInputReceipt) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	context, e := m.beginValueInput(receipt)
	if e != nil {
		return e
	}
	defer func() { m.finishValueInput(cause) }()
	raw := math.Float32bits(value)
	sign := raw>>31 != 0
	exp := int64(int32((raw&0x7f800000)>>23) - 127)
	sig := raw & 0x007fffff
	m.traceSetNativeInput(float64(value), 32, uint64(raw), uint64(sig), sign, exp, 160)
	if e := mpfSourceAssertion(-127 <= exp && exp <= 128, 163, "-127 <= e && e <= +128"); e != nil {
		return e
	}
	out.assignEbits(ebits)
	out.assignSbits(sbits)
	out.assignSign(sign)
	if e := m.assignNativeInputExponent(out, ebits, exp); e != nil {
		return e
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e := m.integer.setUint32WithAllocationContext(&out.significand, sig, context); e != nil {
		return e
	}
	if sbits < 24 {
		if e := m.integer.MachineDiv2kInPlace(&out.significand, 24-sbits); e != nil {
			return e
		}
	} else if sbits > 24 {
		if e := m.integer.mul2kCoreWithAllocationContext(&out.significand, sbits-24, context); e != nil {
			return e
		}
	}
	m.valueExecuting = false
	return m.traceSetFloatRawResult(out)
}

// The source predicates use the original uint32 argument at this expression,
// before any significand SET. ebits=0/64+ reach an undefined signed-shift or
// subtraction domain; the Go guard preserves already-published format/sign.
func (m *ArbitraryFloatManager) assignNativeInputExponent(out *ArbitraryFloat, ebits uint32, exp int64) error {
	if ebits == 0 || ebits >= 64 {
		return arena.ErrBounds
	}
	upper := int64(1) << (ebits - 1)
	if exp <= -(upper - 1) {
		bottom, e := m.bottomExponent(ebits)
		if e != nil {
			return e
		}
		out.exponent = bottom
	} else if exp >= upper {
		top, e := m.topExponent(ebits)
		if e != nil {
			return e
		}
		out.exponent = top
	} else {
		out.exponent = exp
	}
	return nil
}
