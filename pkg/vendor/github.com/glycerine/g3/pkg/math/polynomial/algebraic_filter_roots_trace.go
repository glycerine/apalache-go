package polynomial

// Source: algebraic_numbers.cpp:2235,2248,2250,2262.

import (
	"io"
	"strconv"
)

func (m *AlgebraicRootCore) traceFilterRoots(roots *AlgebraicVector, x uint32, after bool) error {
	line := 2235
	if after {
		line = 2262
	}
	return m.emitRootTraceTag("isolate_roots", "filter_roots", line, func(w io.Writer) error {
		out := algebraicPredicateTraceStream{w}
		text := "before filtering roots, x: x" + strconv.FormatUint(uint64(x), 10) + "\n"
		if after {
			text = "after filtering roots:\n"
		}
		_ = out.AppendCString(text)
		for i := 0; i < len(roots.values); i++ {
			if e := m.traceIsolationRoot(out, &roots.values[i]); e != nil {
				return e
			}
			_ = out.AppendCString("\n")
		}
		return nil
	})
}
func (m *AlgebraicRootCore) traceFilterRootCandidate(i, x uint32, a *Algebraic) error {
	return m.emitRootTraceTag("isolate_roots", "filter_roots", 2248, func(w io.Writer) error {
		out := algebraicPredicateTraceStream{w}
		_ = out.AppendCString("filter_roots i: " + strconv.FormatUint(uint64(i), 10) + ", ext_x2v: x" + strconv.FormatUint(uint64(x), 10) + " -> ")
		if e := m.traceIsolationRoot(out, a); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}
func (m *AlgebraicRootCore) traceFilterRootSign(i uint32, sign int) error {
	return m.emitRootTraceTag("isolate_roots", "filter_roots", 2250, func(w io.Writer) error {
		return (algebraicPredicateTraceStream{w}).AppendCString("filter_roots i: " + strconv.FormatUint(uint64(i), 10) + ", result sign: " + strconv.Itoa(sign) + "\n")
	})
}
