package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:371-373,1817-1823.

import (
	"io"
	"strconv"
)

// sign_lower reads the existing root cell's packed sign only at its reached
// source operand. No polynomial, bound exponent or numeral is loaded here.
func (m *AlgebraicRootCore) comparisonLowerSign(a *Algebraic) (int32, error) {
	l, e := m.locationCleanup(a)
	if e != nil {
		return 0, e
	}
	if l == nil || l.kind != 1 {
		return 0, ErrAlgebraicOwner
	}
	flags, e := m.config.Allocator.ReadUint32(l.raw, 16)
	if e != nil {
		return 0, e
	}
	if flags&2 != 0 {
		return -1, nil
	}
	return 1, nil
}

// Source: algebraic_numbers.cpp:1817-1818, after both sign variations and
// before the V==0 exit, while the actual scoped sequence remains alive.
func (m *AlgebraicRootCore) traceComparisonSturm(a, b *Algebraic, v int32) error {
	return m.emitRootTrace("compare_core", 1817, func(w io.Writer) error {
		out := algebraicComparisonTraceWriter{w}
		_ = writeAlgebraicIntervalPart(out, "comparing using sturm\n")
		for _, value := range [2]*Algebraic{a, b} {
			l, e := m.locationCleanup(value)
			if e != nil {
				return e
			}
			if e = m.displayIntervalAtLocationWithReceipts(out, l, &m.rootTrace.insertions[0], &m.rootTrace.insertions[1]); e != nil {
				return e
			}
			_ = writeAlgebraicIntervalPart(out, "\n")
		}
		_ = writeAlgebraicIntervalPart(out, "V: "+strconv.FormatInt(int64(v), 10)+", sign_lower(a): ")
		sign, e := m.comparisonLowerSign(a)
		if e != nil {
			return e
		}
		_ = writeAlgebraicIntervalPart(out, strconv.FormatInt(int64(sign), 10)+", sign_lower(b): ")
		sign, e = m.comparisonLowerSign(b)
		if e != nil {
			return e
		}
		return writeAlgebraicIntervalPart(out, strconv.FormatInt(int64(sign), 10)+"\n")
	})
}
