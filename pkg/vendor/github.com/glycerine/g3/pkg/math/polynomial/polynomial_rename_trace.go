package polynomial

// Source: src/math/polynomial/polynomial.cpp:6071-6078,6091-6098.

import "io"

func (m *MultivariatePolynomialManager) traceRename(xs []uint32, before bool) error {
	tags := m.core.monomials.monomialTraceTags()
	if !tags.ShouldTrace("rename") {
		return nil
	}
	c := m.core
	c.renameTraceRunning = true
	defer func() { c.renameTraceRunning = false }()
	line := 6091
	if before {
		line = 6071
	}
	var formatErr error
	_ = tags.EmitConditional("rename", "rename", "src/math/polynomial/polynomial.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			if before {
				for _, x := range xs {
					_ = sink.WriteUnsigned(x)
					_ = sink.WriteLiteral(" ")
				}
				_ = sink.WriteLiteral("\npolynomials before rename\n")
			} else {
				_ = sink.WriteLiteral("polynomials after rename\n")
			}
			for _, p := range c.polynomials {
				if p == (Polynomial{}) {
					continue
				}
				formatErr = c.displayPolynomialDirect(p, sink, false)
				if formatErr != nil {
					return formatErr
				}
				_ = sink.WriteLiteral("\n")
			}
			return nil
		})
	return formatErr
}
