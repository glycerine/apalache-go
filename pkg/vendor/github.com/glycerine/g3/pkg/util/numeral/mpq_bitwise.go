package numeral

// Source: src/util/mpq.h bitwise forwards, preserving numerator action before
// output-denominator reset. Arithmetic remains the concrete integer algorithm.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func (m *RationalManager) BitwiseOr(a, b, out *Rational) error {
	return m.rationalBitwise(a, b, out, 0)
}
func (m *RationalManager) BitwiseAnd(a, b, out *Rational) error {
	return m.rationalBitwise(a, b, out, 1)
}
func (m *RationalManager) BitwiseXor(a, b, out *Rational) error {
	return m.rationalBitwise(a, b, out, 2)
}
func (m *RationalManager) rationalBitwise(a, b, out *Rational, op uint8) error {
	return m.rationalBitwiseWithAllocationContext(a, b, out, op, integerAllocationInvocationContext{})
}
func (m *RationalManager) rationalBitwiseWithAllocationContext(a, b, out *Rational, op uint8, context integerAllocationInvocationContext) error {
	line := int32(635)
	switch op {
	case 1:
		line = 643
	case 2:
		line = 651
	}
	if e := m.integerOperandsAt(a, b, line); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	e := m.Z.bitwiseBinaryWithAllocationContext(&a.Num, &b.Num, &out.Num, op, context)
	if e != nil {
		return e
	}
	return m.resetDenominator(out)
}
func (m *RationalManager) BitwiseNot(width uint32, a, out *Rational) error {
	return m.bitwiseNotWithAllocationContext(width, a, out, integerAllocationInvocationContext{})
}
func (m *RationalManager) bitwiseNotWithAllocationContext(width uint32, a, out *Rational, context integerAllocationInvocationContext) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled && !isIntegerRational(a) {
			if e := diagnostic.DebugAssertionFailure(d, "src/util/mpq.h", 659, "is_int(a)"); e != nil {
				return e
			}
		}
	}
	if e := m.Z.bitwiseNotWithAllocationContext(width, &a.Num, &out.Num, context); e != nil {
		return e
	}
	return m.resetDenominator(out)
}
