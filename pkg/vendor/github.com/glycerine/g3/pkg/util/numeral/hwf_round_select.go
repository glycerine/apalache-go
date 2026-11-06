package numeral

// Source: src/util/hwf.cpp:301-355 (Linux path).

import "math"

// RoundToIntegral follows the active source rounding mode without changing
// the process-wide FPU environment. The result remains binary64.
func (m *HardwareFloatManager) RoundToIntegral(mode ArbitraryFloatRoundingMode, in, out *HardwareFloat) error {
	if m == nil || in == nil || out == nil {
		return ErrNumericOwner
	}
	mode, e := hwfSourceRoundingMode(mode)
	if e != nil {
		return e
	}
	x := in.value
	switch mode {
	case ArbitraryFloatNearestEven:
		out.value = math.RoundToEven(x)
	case ArbitraryFloatTowardPositive:
		out.value = math.Ceil(x)
	case ArbitraryFloatTowardNegative:
		out.value = math.Floor(x)
	case ArbitraryFloatTowardZero:
		out.value = math.Trunc(x)
	}
	return nil
}
