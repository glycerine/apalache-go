package numeral

// Source: src/util/f2n.h:24-156, instantiated as f2n<hwf_manager> by
// src/math/subpaving/tactic/subpaving_tactic.cpp:60.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrHardwareFloatNonRegular = errors.New("numeral: non-regular hardware float")

// HardwareFloatNumeralManager is the concrete subpaving f2n<hwf_manager>
// wrapper. Its scratch values are scalar binary64 values with no owned payload.
type HardwareFloatNumeralManager struct {
	core         *HardwareFloatManager
	mode         ArbitraryFloatRoundingMode
	ebits, sbits uint32
	tmp, one     HardwareFloat
}

func NewHardwareFloatNumeralManager(core *HardwareFloatManager, ebits, sbits uint32) (*HardwareFloatNumeralManager, error) {
	if core == nil {
		return nil, ErrNumericOwner
	}
	return &HardwareFloatNumeralManager{core: core, mode: ArbitraryFloatTowardPositive, ebits: ebits, sbits: sbits, one: HardwareFloat{1}}, nil
}

func (m *HardwareFloatNumeralManager) check(out *HardwareFloat) error {
	if m == nil || m.core == nil || out == nil {
		return ErrNumericOwner
	}
	if !m.core.IsRegular(out) {
		return ErrHardwareFloatNonRegular
	}
	return nil
}

func (m *HardwareFloatNumeralManager) RoundingMode() ArbitraryFloatRoundingMode { return m.mode }
func (m *HardwareFloatNumeralManager) ExponentBits() uint32                     { return m.ebits }
func (m *HardwareFloatNumeralManager) SignificandBits() uint32                  { return m.sbits }
func (m *HardwareFloatNumeralManager) RoundToPlusInf()                          { m.mode = ArbitraryFloatTowardPositive }
func (m *HardwareFloatNumeralManager) RoundToMinusInf()                         { m.mode = ArbitraryFloatTowardNegative }
func (m *HardwareFloatNumeralManager) SetRounding(toPlus bool) {
	if toPlus {
		m.RoundToPlusInf()
	} else {
		m.RoundToMinusInf()
	}
}

func (m *HardwareFloatNumeralManager) SetInt32(out *HardwareFloat, value int32) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if err := m.core.SetInt32(out, value); err != nil {
		return err
	}
	return m.check(out)
}
func (m *HardwareFloatNumeralManager) SetUint32(out *HardwareFloat, value uint32) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if err := m.core.SetFloat64(out, float64(value)); err != nil {
		return err
	}
	return m.check(out)
}
func (m *HardwareFloatNumeralManager) SetFloat64(out *HardwareFloat, value float64) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if err := m.core.SetFloat64(out, value); err != nil {
		return err
	}
	return m.check(out)
}
func (m *HardwareFloatNumeralManager) Set(out, in *HardwareFloat) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if err := m.core.Copy(out, in); err != nil {
		return err
	}
	return m.check(out)
}
func (m *HardwareFloatNumeralManager) Reset(out *HardwareFloat) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	return m.core.Reset(out)
}
func (m *HardwareFloatNumeralManager) IsZero(a *HardwareFloat) bool { return m.core.IsZero(a) }
func (m *HardwareFloatNumeralManager) IsNeg(a *HardwareFloat) bool {
	return m.core.IsNeg(a) && !m.core.IsZero(a)
}
func (m *HardwareFloatNumeralManager) IsPos(a *HardwareFloat) bool {
	return m.core.IsPos(a) && !m.core.IsZero(a)
}
func (m *HardwareFloatNumeralManager) IsNonNeg(a *HardwareFloat) bool { return !m.IsNeg(a) }
func (m *HardwareFloatNumeralManager) IsNonPos(a *HardwareFloat) bool { return !m.IsPos(a) }
func (m *HardwareFloatNumeralManager) IsOne(a *HardwareFloat) bool    { return m.core.IsOne(a) }
func (m *HardwareFloatNumeralManager) IsMinusOne(a *HardwareFloat) bool {
	// Source temporarily negates the borrowed value, tests one, then restores it.
	return a.RawBits() == 0xbff0000000000000
}
func (m *HardwareFloatNumeralManager) IsInteger(a *HardwareFloat) bool { return m.core.IsInteger(a) }
func (m *HardwareFloatNumeralManager) Equal(a, b *HardwareFloat) bool  { return m.core.Equal(a, b) }
func (m *HardwareFloatNumeralManager) Less(a, b *HardwareFloat) bool   { return m.core.Less(a, b) }
func (m *HardwareFloatNumeralManager) LessEqual(a, b *HardwareFloat) bool {
	return m.core.LessEqual(a, b)
}
func (m *HardwareFloatNumeralManager) Greater(a, b *HardwareFloat) bool { return m.core.Greater(a, b) }
func (m *HardwareFloatNumeralManager) GreaterEqual(a, b *HardwareFloat) bool {
	return m.core.GreaterEqual(a, b)
}
func (m *HardwareFloatNumeralManager) ToFloat64(a *HardwareFloat) float64 { return m.core.ToFloat64(a) }
func (m *HardwareFloatNumeralManager) Abs(in, out *HardwareFloat) error   { return m.core.Abs(out, in) }
func (m *HardwareFloatNumeralManager) Neg(in, out *HardwareFloat) error   { return m.core.Neg(out, in) }
func (m *HardwareFloatNumeralManager) Swap(a, b *HardwareFloat) error     { return m.core.Swap(a, b) }

func (m *HardwareFloatNumeralManager) Add(x, y, out *HardwareFloat) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if err := m.core.Add(m.mode, x, y, out); err != nil {
		return err
	}
	return m.check(out)
}
func (m *HardwareFloatNumeralManager) Sub(x, y, out *HardwareFloat) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if err := m.core.Sub(m.mode, x, y, out); err != nil {
		return err
	}
	return m.check(out)
}
func (m *HardwareFloatNumeralManager) Mul(x, y, out *HardwareFloat) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if err := m.core.Mul(m.mode, x, y, out); err != nil {
		return err
	}
	return m.check(out)
}
func (m *HardwareFloatNumeralManager) Div(x, y, out *HardwareFloat) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if err := m.core.Div(m.mode, x, y, out); err != nil {
		return err
	}
	return m.check(out)
}
func (m *HardwareFloatNumeralManager) Inv(out *HardwareFloat) error { return m.Div(&m.one, out, out) }
func (m *HardwareFloatNumeralManager) Inc(out *HardwareFloat) error { return m.Add(out, &m.one, out) }
func (m *HardwareFloatNumeralManager) Dec(out *HardwareFloat) error { return m.Sub(out, &m.one, out) }

func (m *HardwareFloatNumeralManager) Floor(in, out *HardwareFloat) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if err := m.core.RoundToIntegral(ArbitraryFloatTowardNegative, in, &m.tmp); err != nil {
		return err
	}
	if m.core.LessEqual(&m.tmp, in) {
		return m.Set(out, &m.tmp)
	}
	return m.core.Sub(ArbitraryFloatTowardNegative, &m.tmp, &m.one, out)
}
func (m *HardwareFloatNumeralManager) Ceil(in, out *HardwareFloat) error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if err := m.core.RoundToIntegral(ArbitraryFloatTowardPositive, in, &m.tmp); err != nil {
		return err
	}
	if m.core.GreaterEqual(&m.tmp, in) {
		return m.Set(out, &m.tmp)
	}
	return m.core.Add(ArbitraryFloatTowardNegative, &m.tmp, &m.one, out)
}

func (m *HardwareFloatNumeralManager) Power(a *HardwareFloat, exponent uint32, out *HardwareFloat) error {
	if m == nil || m.core == nil || a == nil || out == nil {
		return ErrNumericOwner
	}
	// Source set(power, a) checks the copied input before set(b, 1).
	if err := m.check(a); err != nil {
		return err
	}
	// The source's unsigned mask loop wraps for exponents >= 2^31.
	if exponent >= 1<<31 {
		return arena.ErrBounds
	}
	power := *a
	if err := m.SetInt32(out, 1); err != nil {
		return err
	}
	for mask := uint32(1); mask <= exponent; mask <<= 1 {
		if mask&exponent != 0 {
			if err := m.Mul(out, &power, out); err != nil {
				return err
			}
		}
		if err := m.Mul(&power, &power, &power); err != nil {
			return err
		}
	}
	return m.check(out)
}
