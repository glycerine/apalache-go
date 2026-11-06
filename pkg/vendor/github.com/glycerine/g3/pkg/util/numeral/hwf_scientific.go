package numeral

// Source: src/util/hwf.cpp:380-384; src/util/f2n.h:163,165.

import (
	"io"
	"math"
	"strconv"
)

// ToScientificString follows the source stream's scientific notation and
// default six digits after the decimal point for finite binary64 values.
func (m *HardwareFloatManager) ToScientificString(x *HardwareFloat) (string, error) {
	if m == nil || x == nil {
		return "", ErrNumericOwner
	}
	if math.IsNaN(x.value) {
		if math.Signbit(x.value) {
			return "-nan", nil
		}
		return "nan", nil
	}
	if math.IsInf(x.value, 1) {
		return "inf", nil
	}
	if math.IsInf(x.value, -1) {
		return "-inf", nil
	}
	return strconv.FormatFloat(x.value, 'e', 6, 64), nil
}

func (m *HardwareFloatNumeralManager) ToString(x *HardwareFloat) (string, error) {
	if m == nil || m.core == nil {
		return "", ErrNumericOwner
	}
	return m.core.ToScientificString(x)
}

func (m *HardwareFloatNumeralManager) DisplayTo(w io.Writer, x *HardwareFloat) error {
	if w == nil {
		return ErrNumericOwner
	}
	s, e := m.ToString(x)
	if e != nil {
		return e
	}
	n, e := io.WriteString(w, s)
	if e != nil {
		return e
	}
	if n != len(s) {
		return io.ErrShortWrite
	}
	return nil
}
