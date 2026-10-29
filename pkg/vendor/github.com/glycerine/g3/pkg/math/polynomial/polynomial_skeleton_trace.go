package polynomial

// Source: src/math/polynomial/polynomial.cpp:3012-3016; src/util/trace.h:40-46.

import "io"

func (m *MonomialManager) traceSkeletonLexCompare(a, b Monomial, min uint32) error {
	tags := m.monomialTraceTags()
	if !tags.ShouldTrace("lex_bug") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("lex_bug", "operator()", "src/math/polynomial/polynomial.cpp", 684,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("min: x")
			_ = sink.WriteUnsigned(min)
			_ = sink.WriteLiteral("\n")
			formatErr = m.DisplayInto(a, sink, false)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\n")
			formatErr = m.DisplayInto(b, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (s *PolynomialSkeleton) traceConstruction(p Polynomial) error {
	m := s.owner
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("skeleton") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("skeleton", "skeleton", "src/math/polynomial/polynomial.cpp", 3012,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("x: x")
			_ = sink.WriteUnsigned(s.x)
			_ = sink.WriteLiteral("\nmax: ")
			_ = sink.WriteUnsigned(s.maxPowers)
			_ = sink.WriteLiteral("\np: ")
			formatErr = m.displayPolynomialDirect(p, sink, false)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\nskeleton: ")
			for i, entry := range s.entries {
				if i > 0 {
					_ = sink.WriteLiteral(" ")
				}
				_ = sink.WriteLiteral("(")
				for j := uint32(0); j < entry.NumPowers; j++ {
					if j > 0 {
						_ = sink.WriteLiteral(" ")
					}
					_ = sink.WriteVariable(s.x)
					_ = sink.WriteLiteral("^")
					_ = sink.WriteUnsigned(s.powers[entry.FirstPower+j])
				}
				_ = sink.WriteLiteral(")*")
				formatErr = m.monomials.DisplayInto(entry.Monomial, sink, false)
				if formatErr != nil {
					return formatErr
				}
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}
