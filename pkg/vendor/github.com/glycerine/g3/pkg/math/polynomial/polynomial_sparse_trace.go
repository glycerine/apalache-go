package polynomial

// Source: src/math/polynomial/polynomial.cpp:3157-3171; src/util/trace.h:40-46.

import (
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (s *PolynomialSparseInterpolator) traceEquation(cs []unumeral.Int) error {
	m := s.owner.core
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("sparse_interpolator") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("sparse_interpolator", "mk", "src/math/polynomial/polynomial.cpp", 3157,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("adding new equation:\n")
			for i := range cs {
				value, err := m.modular.String(&cs[i])
				if err != nil {
					formatErr = err
					return err
				}
				_ = sink.WriteLiteral(value)
				_ = sink.WriteLiteral(" ")
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (s *PolynomialSparseInterpolator) traceSystem(entry PolynomialSkeletonEntry, solver *ModularLinearSolver) error {
	m := s.owner.core
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("sparse_interpolator") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("sparse_interpolator", "mk", "src/math/polynomial/polynomial.cpp", 3164,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("find coefficients of:\n")
			for i := uint32(0); i < entry.NumPowers; i++ {
				term, err := s.skeleton.Original(entry, i)
				if err != nil {
					formatErr = err
					return err
				}
				formatErr = m.monomials.DisplayInto(term, sink, false)
				if formatErr != nil {
					return formatErr
				}
				_ = sink.WriteLiteral("\n")
			}
			_ = sink.WriteLiteral("system of equations:\n")
			formatErr = solver.Display(sink)
			return formatErr
		})
	return formatErr
}
