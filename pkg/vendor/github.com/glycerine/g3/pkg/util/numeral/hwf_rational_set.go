package numeral

// Source: src/util/hwf.cpp:101-105,114-117; src/util/mpq.cpp:297-303;
// src/util/mpz.cpp:1364-1380 (non-GMP branch).

import "math"

// integerFloat64Mode repeats the source's UINT_MAX-radix conversion. Each
// floating multiply and add observes the HWF rounding mode, as it does after
// the source changes the FPU mode before mpq_manager::get_double.
func (m *HardwareFloatManager) integerFloat64Mode(mode ArbitraryFloatRoundingMode, z *IntegerManager, a *Int) (HardwareFloat, error) {
	if e := z.check(a); e != nil {
		return HardwareFloat{}, e
	}
	if a.IsSmall() {
		return HardwareFloat{float64(a.Val)}, nil
	}
	r, d := HardwareFloat{}, HardwareFloat{1}
	radix := HardwareFloat{float64(^uint32(0))}
	for i, count := uint32(0), z.Cell(a.Cell).Size; i < count; i++ {
		digit := HardwareFloat{float64(z.Digit(a.Cell, i))}
		var product HardwareFloat
		if e := m.Mul(mode, &d, &digit, &product); e != nil {
			return HardwareFloat{}, e
		}
		if e := m.Add(mode, &r, &product, &r); e != nil {
			return HardwareFloat{}, e
		}
		if e := m.Mul(mode, &d, &radix, &d); e != nil {
			return HardwareFloat{}, e
		}
	}
	if a.Val < 0 {
		r.value = math.Float64frombits(r.RawBits() ^ (uint64(1) << 63))
	}
	return r, nil
}

func (m *HardwareFloatManager) SetRatioInt32(out *HardwareFloat, mode ArbitraryFloatRoundingMode, n, d int32) error {
	if m == nil || out == nil {
		return ErrNumericOwner
	}
	a, b := HardwareFloat{float64(n)}, HardwareFloat{float64(d)}
	return m.Div(mode, &a, &b, out)
}

// SetRational follows the source's two integer projections and final division.
// The rational owner is borrowed for this call and the HWF output owns no data.
func (m *HardwareFloatManager) SetRational(out *HardwareFloat, mode ArbitraryFloatRoundingMode, qm *RationalManager, value *Rational) error {
	if m == nil || out == nil || qm == nil {
		return ErrNumericOwner
	}
	mode, e := hwfSourceRoundingMode(mode)
	if e != nil {
		return e
	}
	if e := qm.checkScalarValue(value); e != nil {
		return e
	}
	n, e := m.integerFloat64Mode(mode, qm.Z, &value.Num)
	if e != nil {
		return e
	}
	d, e := m.integerFloat64Mode(mode, qm.Z, &value.Den)
	if e != nil {
		return e
	}
	if modularSourceDebug && !(d.value > 0) {
		return ErrRationalFloatDomain
	}
	return m.Div(mode, &n, &d, out)
}

func (m *HardwareFloatNumeralManager) SetRatioInt32(out *HardwareFloat, n, d int32) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if e := m.core.SetRatioInt32(out, m.mode, n, d); e != nil {
		return e
	}
	return m.check(out)
}

func (m *HardwareFloatNumeralManager) SetRational(out *HardwareFloat, qm *RationalManager, value *Rational) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if e := m.core.SetRational(out, m.mode, qm, value); e != nil {
		return e
	}
	return m.check(out)
}
