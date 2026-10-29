package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1123-1126.

import (
	"io"
	"strconv"
)

func (m *AlgebraicRootCore) traceUnaryIntervals(a *Algebraic, lv, uv uint32) error {
	return m.emitRootTrace("mk_unary", 1123, func(w io.Writer) error {
		out := algebraicComparisonTraceWriter{w}
		_ = writeAlgebraicIntervalPart(out, "r_i: ")
		if e := m.config.Intervals.Display(out, &m.binary.interval); e != nil {
			return e
		}
		_ = writeAlgebraicIntervalPart(out, "\nlV: "+strconv.FormatUint(uint64(lv), 10)+", uV: "+strconv.FormatUint(uint64(uv), 10)+"\na.m_interval: ")
		l, e := m.locationCleanup(a)
		if e != nil {
			return e
		}
		r, e := m.load(l)
		if e != nil {
			return e
		}
		if e = m.config.Intervals.Display(out, &r.interval); e != nil {
			return e
		}
		return writeAlgebraicIntervalPart(out, "\n")
	})
}
