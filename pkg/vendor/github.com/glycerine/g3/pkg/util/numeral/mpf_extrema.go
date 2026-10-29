package numeral

// Source: src/util/mpf.cpp:1487-1517; src/util/mpf.h:119,122,137-138.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func (m *ArbitraryFloatManager) Le(x, y *ArbitraryFloat) (bool, error) { return m.Lte(x, y) }
func (m *ArbitraryFloatManager) Ge(x, y *ArbitraryFloat) (bool, error) { return m.Gte(x, y) }
func (m *ArbitraryFloatManager) Maximum(x, y, out *ArbitraryFloat) error {
	return m.MaximumWithReceipt(x, y, out, nil)
}
func (m *ArbitraryFloatManager) Minimum(x, y, out *ArbitraryFloat) error {
	return m.MinimumWithReceipt(x, y, out, nil)
}
func (m *ArbitraryFloatManager) MaximumWithReceipt(x, y, out *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt) error {
	return m.extremum(x, y, out, true, receipt)
}
func (m *ArbitraryFloatManager) MinimumWithReceipt(x, y, out *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt) error {
	return m.extremum(x, y, out, false, receipt)
}
func (m *ArbitraryFloatManager) extremum(x, y, out *ArbitraryFloat, maximum bool, receipt *ArbitraryFloatValueCopyReceipt) error {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	nan, e := m.IsNaN(x)
	if e != nil {
		return e
	}
	if nan {
		return m.CopyWithReceipt(out, y, receipt)
	}
	nan, e = m.IsNaN(y)
	if e != nil {
		return e
	}
	if nan {
		return m.CopyWithReceipt(out, x, receipt)
	}
	zero, e := m.bothZero(x, y)
	if e != nil {
		return e
	}
	if zero && x.Sign() != y.Sign() {
		line := int32(1509)
		if maximum {
			line = 1493
		}
		return diagnostic.BorrowProcessDebugContext().UnreachableSource("src/util/mpf.cpp", line)
	} // Source UNREACHABLE; no chosen signed-zero result.
	// The source evaluates the zero predicates again in this separate else-if.
	zero, e = m.bothZero(x, y)
	if e != nil {
		return e
	}
	if zero {
		return m.CopyWithReceipt(out, y, receipt)
	}
	var selected bool
	if maximum {
		selected, e = m.Gt(x, y)
	} else {
		selected, e = m.Lt(x, y)
	}
	if e != nil {
		return e
	}
	if selected {
		return m.CopyWithReceipt(out, x, receipt)
	}
	return m.CopyWithReceipt(out, y, receipt)
}
