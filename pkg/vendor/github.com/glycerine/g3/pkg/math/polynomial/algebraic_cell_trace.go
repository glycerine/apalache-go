package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:506.

import "io"

// set's linear scoped_mpq has already destructed; both branches have assigned a.
// mk_algebraic_cell has no corresponding record and uses the private constructor.
func (m *AlgebraicRootCore) traceSetRoot(a *Algebraic) error {
	return m.emitRootTrace("set", 506, func(w io.Writer) error {
		out := algebraicPredicateTraceStream{w}
		_ = out.AppendCString("a: ")
		l, e := m.locationCleanup(a)
		if e != nil {
			return e
		}
		if e = m.displayRootAtLocationWithReceipt(out, l, false, &m.rootTrace.root); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}
