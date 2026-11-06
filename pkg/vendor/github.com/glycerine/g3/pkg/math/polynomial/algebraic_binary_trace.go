package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:945,951,984,1012-1068.

import (
	"io"
	"strconv"
)

func (m *AlgebraicRootCore) traceSetCorePolynomial(p UPolynomialView) error {
	return m.emitRootTrace("set_core", 945, func(w io.Writer) error {
		out := upolyRemainderTraceStream{w}
		_ = out.AppendCString("set_core p: ")
		if e := m.config.Polynomials.displayIntoWithReceipt(p, out, "x", false, &m.rootTrace.univariate); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}

func (m *AlgebraicRootCore) traceSetCoreReset(a *Algebraic) error {
	return m.emitRootTrace("set_core", 951, func(w io.Writer) error {
		out := algebraicPredicateTraceStream{w}
		_ = out.AppendCString("reseting\nresult: ")
		l, e := m.locationCleanup(a)
		if e != nil {
			return e
		}
		if e = m.displayRootAtLocation(out, l, false); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}

func (m *AlgebraicRootCore) traceSetCoreAlgebraic() error {
	return m.emitRootTrace("set_core", 984, func(w io.Writer) error {
		return (algebraicPredicateTraceStream{w}).AppendCString("set_core...\n")
	})
}

func (m *AlgebraicRootCore) traceBinaryIntervals(a, b *Algebraic, lv, uv uint32) error {
	return m.emitRootTrace("mk_binary", 1046, func(w io.Writer) error {
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
		_ = writeAlgebraicIntervalPart(out, "\nb.m_interval: ")
		l, e = m.locationCleanup(b)
		if e != nil {
			return e
		}
		r, e = m.load(l)
		if e != nil {
			return e
		}
		if e = m.config.Intervals.Display(out, &r.interval); e != nil {
			return e
		}
		return writeAlgebraicIntervalPart(out, "\n")
	})
}

func (m *AlgebraicRootCore) traceBinaryOperands(a, b *Algebraic) error {
	return m.emitRootTraceTag("anum_mk_binary", "mk_binary", 1012, func(w io.Writer) error {
		out := algebraicPredicateTraceStream{w}
		for i, value := range [2]*Algebraic{a, b} {
			label := "a: "
			if i != 0 {
				label = "\nb: "
			}
			_ = out.AppendCString(label)
			l, e := m.locationCleanup(value)
			if e != nil {
				return e
			}
			if m.rootDisplay.active {
				return ErrAlgebraicPending
			}
			e = m.displayRootAtLocation(out, l, false)
			if m.rootDisplay.active {
				m.binary.traceRootPending = true
				m.binary.traceRootStop = m.rootDisplay.sourceStop
			}
			if e != nil {
				return e
			}
		}
		_ = out.AppendCString("\np: ")
		p, e := m.config.Polynomials.wholeView(m.binary.p)
		if e != nil {
			return e
		}
		if e = m.config.Polynomials.displayIntoWithReceipt(p, out, "x", false, &m.rootTrace.univariate); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}

func (m *AlgebraicRootCore) traceBinaryFactor(index uint32) error {
	return m.emitRootTraceTag("anum_mk_binary", "mk_binary", 1019, func(w io.Writer) error {
		out := upolyRemainderTraceStream{w}
		_ = out.AppendCString("factor " + strconv.FormatUint(uint64(index), 10) + "\n")
		p, e := m.binary.factors.FactorView(index)
		if e != nil {
			return e
		}
		if e = m.config.Polynomials.displayIntoWithReceipt(p, out, "x", false, &m.rootTrace.univariate); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}

func (m *AlgebraicRootCore) traceBinarySequence(index uint32, seq *UPolynomialSequence) error {
	return m.emitRootTraceTag("anum_mk_binary", "mk_binary", 1042, func(w io.Writer) error {
		out := upolyRemainderTraceStream{w}
		_ = out.AppendCString("sequence " + strconv.FormatUint(uint64(index), 10) + "\n")
		if e := m.config.Polynomials.displaySequenceIntoWithReceipt(seq, out, "x", &m.rootTrace.univariate); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}

func (m *AlgebraicRootCore) traceBinaryTarget(index uint32) error {
	return m.emitRootTraceTag("anum_mk_binary", "mk_binary", 1068, func(w io.Writer) error {
		return (algebraicPredicateTraceStream{w}).AppendCString("target_i: " + strconv.FormatUint(uint64(index), 10) + "\n")
	})
}
