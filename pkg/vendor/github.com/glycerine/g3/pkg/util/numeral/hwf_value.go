package numeral

// Source: src/util/hwf.h:26-44, 45-62, 126-160;
// src/util/hwf.cpp:97-110,168-214,430-555.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

// HardwareFloat is the concrete binary64 numeral used by the retained
// subpaving hardware-float engine. Its bits are scalar and own no resource.
type HardwareFloat struct{ value float64 }

type HardwareFloatManager struct{}

func NewHardwareFloatManager() *HardwareFloatManager { return &HardwareFloatManager{} }

func (a HardwareFloat) RawBits() uint64 { return math.Float64bits(a.value) }

func (m *HardwareFloatManager) SetInt32(out *HardwareFloat, value int32) error {
	if m == nil || out == nil {
		return ErrNumericOwner
	}
	out.value = float64(value)
	return nil
}

func (m *HardwareFloatManager) SetFloat32(out *HardwareFloat, value float32) error {
	if m == nil || out == nil {
		return ErrNumericOwner
	}
	out.value = float64(value)
	return nil
}

func (m *HardwareFloatManager) SetFloat64(out *HardwareFloat, value float64) error {
	if m == nil || out == nil {
		return ErrNumericOwner
	}
	out.value = value
	return nil
}

func (m *HardwareFloatManager) Copy(out, in *HardwareFloat) error {
	if m == nil || out == nil || in == nil {
		return ErrNumericOwner
	}
	out.value = in.value
	return nil
}

func (m *HardwareFloatManager) Reset(out *HardwareFloat) error {
	return m.SetInt32(out, 0)
}

func (m *HardwareFloatManager) SetComponents(out *HardwareFloat, sign bool, significand uint64, exponent int32) error {
	if m == nil || out == nil {
		return ErrNumericOwner
	}
	if significand > (uint64(1)<<52)-1 || exponent < -1022 || exponent > 1023 {
		return arena.ErrBounds
	}
	raw := uint64(exponent+1023)<<52 | significand
	if sign {
		raw |= uint64(1) << 63
	}
	out.value = math.Float64frombits(raw)
	return nil
}

func (m *HardwareFloatManager) MkZero(out *HardwareFloat, sign bool) error {
	if m == nil || out == nil {
		return ErrNumericOwner
	}
	if sign {
		out.value = math.Float64frombits(uint64(1) << 63)
	} else {
		out.value = 0
	}
	return nil
}

func (m *HardwareFloatManager) MkNaN(out *HardwareFloat) error {
	if m == nil || out == nil {
		return ErrNumericOwner
	}
	out.value = math.Float64frombits(0x7ff0000000000001)
	return nil
}

func (m *HardwareFloatManager) MkInf(out *HardwareFloat, sign bool) error {
	if m == nil || out == nil {
		return ErrNumericOwner
	}
	raw := uint64(0x7ff0000000000000)
	if sign {
		raw |= uint64(1) << 63
	}
	out.value = math.Float64frombits(raw)
	return nil
}

func (m *HardwareFloatManager) Swap(a, b *HardwareFloat) error {
	if m == nil || a == nil || b == nil {
		return ErrNumericOwner
	}
	a.value, b.value = b.value, a.value
	return nil
}

func (m *HardwareFloatManager) ToFloat64(a *HardwareFloat) float64 {
	return a.value
}

func (m *HardwareFloatManager) Abs(out, in *HardwareFloat) error {
	if m == nil || out == nil || in == nil {
		return ErrNumericOwner
	}
	out.value = math.Float64frombits(in.RawBits() & 0x7fffffffffffffff)
	return nil
}

func (m *HardwareFloatManager) Neg(out, in *HardwareFloat) error {
	if m == nil || out == nil || in == nil {
		return ErrNumericOwner
	}
	out.value = math.Float64frombits(in.RawBits() ^ (uint64(1) << 63))
	return nil
}

func (m *HardwareFloatManager) Equal(a, b *HardwareFloat) bool {
	return a.value == b.value
}
func (m *HardwareFloatManager) Less(a, b *HardwareFloat) bool {
	return a.value < b.value
}
func (m *HardwareFloatManager) LessEqual(a, b *HardwareFloat) bool {
	return a.value <= b.value
}
func (m *HardwareFloatManager) Greater(a, b *HardwareFloat) bool {
	return a.value > b.value
}
func (m *HardwareFloatManager) GreaterEqual(a, b *HardwareFloat) bool {
	return a.value >= b.value
}

func (m *HardwareFloatManager) Sign(a *HardwareFloat) bool {
	return a.RawBits()>>63 != 0
}
func (m *HardwareFloatManager) Significand(a *HardwareFloat) uint64 {
	return a.RawBits() & 0x000fffffffffffff
}
func (m *HardwareFloatManager) Exponent(a *HardwareFloat) int32 {
	return int32((a.RawBits()>>52)&0x7ff) - 1023
}
func (m *HardwareFloatManager) IsZero(a *HardwareFloat) bool {
	return a.RawBits()&0x7fffffffffffffff == 0
}
func (m *HardwareFloatManager) IsNaN(a *HardwareFloat) bool {
	raw := a.RawBits()
	return raw&0x7ff0000000000000 == 0x7ff0000000000000 && raw&0x000fffffffffffff != 0
}
func (m *HardwareFloatManager) IsInf(a *HardwareFloat) bool {
	return a.RawBits()&0x7fffffffffffffff == 0x7ff0000000000000
}
func (m *HardwareFloatManager) IsNeg(a *HardwareFloat) bool {
	return m.Sign(a) && !m.IsNaN(a)
}
func (m *HardwareFloatManager) IsPos(a *HardwareFloat) bool {
	return !m.Sign(a) && !m.IsNaN(a)
}
func (m *HardwareFloatManager) IsNegativeZero(a *HardwareFloat) bool {
	return a.RawBits() == 0x8000000000000000
}
func (m *HardwareFloatManager) IsPositiveZero(a *HardwareFloat) bool {
	return a.RawBits() == 0
}
func (m *HardwareFloatManager) IsOne(a *HardwareFloat) bool {
	return a.RawBits() == 0x3ff0000000000000
}
func (m *HardwareFloatManager) IsPositiveInf(a *HardwareFloat) bool {
	return !m.Sign(a) && m.IsInf(a)
}
func (m *HardwareFloatManager) IsNegativeInf(a *HardwareFloat) bool {
	return m.Sign(a) && m.IsInf(a)
}
func (m *HardwareFloatManager) IsNormal(a *HardwareFloat) bool {
	exp := a.RawBits() & 0x7ff0000000000000
	return exp != 0 && exp != 0x7ff0000000000000
}
func (m *HardwareFloatManager) IsDenormal(a *HardwareFloat) bool {
	raw := a.RawBits()
	return raw&0x7ff0000000000000 == 0 && raw&0x000fffffffffffff != 0
}
func (m *HardwareFloatManager) IsRegular(a *HardwareFloat) bool {
	return a.RawBits()&0x7ff0000000000000 != 0x7ff0000000000000
}
func (m *HardwareFloatManager) IsInteger(a *HardwareFloat) bool {
	if !m.IsNormal(a) {
		return false
	}
	exp := m.Exponent(a)
	if exp >= 52 {
		return true
	}
	if exp < 0 {
		return false
	}
	mask := (uint64(1) << uint32(52-exp)) - 1
	return m.Significand(a)&mask == 0
}
