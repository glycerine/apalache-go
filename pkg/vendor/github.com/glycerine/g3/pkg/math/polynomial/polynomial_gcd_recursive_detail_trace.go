package polynomial

// Source: src/math/polynomial/polynomial.cpp:4100-4207; src/util/trace.h:40-46.

import (
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialManager) emitModGCDRecDetail(line int, body func(*polynomialTraceSink) error) error {
	tags := m.core.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd_detail") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("mgcd_detail", "mod_gcd_rec", "src/math/polynomial/polynomial.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			formatErr = body(&polynomialTraceSink{writer: w})
			return formatErr
		})
	return formatErr
}

func (m *MultivariatePolynomialManager) gcdRecDetailPolynomial(sink *polynomialTraceSink, label string, p Polynomial, sourceDisplay bool) error {
	_ = sink.WriteLiteral(label)
	var err error
	if sourceDisplay {
		// cpp4103-4108,4115-4116 explicitly call polynomial::display(..., true).
		err = m.core.displayPolynomialDirect(p, sink, true)
	} else {
		// Later records insert polynomial_ref values through manager::display.
		err = m.core.displayIntoCore(p, sink, false)
	}
	if err != nil {
		return err
	}
	return sink.WriteLiteral("\n")
}

func (m *MultivariatePolynomialManager) gcdRecDetailNumeral(sink *polynomialTraceSink, label string, n *unumeral.Int) error {
	_ = sink.WriteLiteral(label)
	value, err := m.core.modular.String(n)
	if err != nil {
		return err
	}
	_ = sink.WriteLiteral(value)
	return sink.WriteLiteral("\n")
}

func (m *MultivariatePolynomialManager) traceModGCDRecInitial(idx, x uint32, pu, pv, cu, cv, lu, lv Polynomial, iu, iv *unumeral.Int) error {
	return m.emitModGCDRecDetail(4100, func(s *polynomialTraceSink) error {
		_ = s.WriteLiteral("idx: ")
		_ = s.WriteUnsigned(idx)
		_ = s.WriteLiteral("\nx")
		_ = s.WriteUnsigned(x)
		_ = s.WriteLiteral("\n")
		for _, row := range [...]struct {
			label string
			p     Polynomial
		}{
			{"pp_u = ", pu}, {"pp_v = ", pv}, {"c_u = ", cu}, {"c_v = ", cv}, {"lc_u = ", lu}, {"lc_v = ", lv},
		} {
			if err := m.gcdRecDetailPolynomial(s, row.label, row.p, true); err != nil {
				return err
			}
		}
		if err := m.gcdRecDetailNumeral(s, "ci_u = ", iu); err != nil {
			return err
		}
		return m.gcdRecDetailNumeral(s, "ci_v = ", iv)
	})
}

func (m *MultivariatePolynomialManager) traceModGCDRecContents(cg, lg Polynomial, ig *unumeral.Int) error {
	return m.emitModGCDRecDetail(4114, func(s *polynomialTraceSink) error {
		if err := m.gcdRecDetailPolynomial(s, "c_g = ", cg, true); err != nil {
			return err
		}
		if err := m.gcdRecDetailPolynomial(s, "lc_g = ", lg, true); err != nil {
			return err
		}
		return m.gcdRecDetailNumeral(s, "ci_g = ", ig)
	})
}

func (m *MultivariatePolynomialManager) traceModGCDRecIteration(counter, idx uint32, q Polynomial, degree, minDegree, nextX, qvar uint32) error {
	return m.emitModGCDRecDetail(4149, func(s *polynomialTraceSink) error {
		_ = s.WriteLiteral("counter: ")
		_ = s.WriteUnsigned(counter)
		_ = s.WriteLiteral("\nidx: ")
		_ = s.WriteUnsigned(idx)
		_ = s.WriteLiteral("\n")
		if err := m.gcdRecDetailPolynomial(s, "q: ", q, false); err != nil {
			return err
		}
		_ = s.WriteLiteral("deg_q: ")
		_ = s.WriteUnsigned(degree)
		_ = s.WriteLiteral("\nmin_deg_q: ")
		_ = s.WriteUnsigned(minDegree)
		_ = s.WriteLiteral("\nnext_x: x")
		_ = s.WriteUnsigned(nextX)
		_ = s.WriteLiteral("\nmax_var(q): ")
		_ = s.WriteUnsigned(qvar)
		return s.WriteLiteral("\n")
	})
}

func (m *MultivariatePolynomialManager) traceModGCDRecDetailLiteral(line int, body string) {
	_ = m.core.monomials.monomialTraceTags().Emit("mgcd_detail", "mod_gcd_rec", "src/math/polynomial/polynomial.cpp", line, body)
}

func (m *MultivariatePolynomialManager) traceModGCDRecCandidate(line int, idx uint32, firstLabel string, first Polynomial, secondLabel string, second Polynomial) error {
	return m.emitModGCDRecDetail(line, func(s *polynomialTraceSink) error {
		_ = s.WriteLiteral("idx: ")
		_ = s.WriteUnsigned(idx)
		_ = s.WriteLiteral("\n")
		if err := m.gcdRecDetailPolynomial(s, firstLabel, first, false); err != nil {
			return err
		}
		if secondLabel != "" {
			return m.gcdRecDetailPolynomial(s, secondLabel, second, false)
		}
		return nil
	})
}

func (m *MultivariatePolynomialManager) traceModGCDRecDivides(candidate, pu, pv Polynomial, receipt *polynomialDividesInvocationReceipt) error {
	if !m.core.monomials.monomialTraceTags().ShouldTrace("mgcd_detail") {
		return nil
	}
	return m.emitModGCDRecDetail(4205, func(s *polynomialTraceSink) error {
		for _, row := range [...]struct {
			label string
			p     Polynomial
		}{{"C: ", candidate}, {"pp_u: ", pu}, {"pp_v: ", pv}} {
			if err := m.gcdRecDetailPolynomial(s, row.label, row.p, false); err != nil {
				return err
			}
		}
		for _, row := range [...]struct {
			label string
			p     Polynomial
		}{{"divides(C, pp_u): ", pu}, {"divides(C, pp_v): ", pv}} {
			_ = s.WriteLiteral(row.label)
			ok, err := m.core.dividesWithReceipt(candidate, row.p, receipt)
			if err != nil {
				return err
			}
			if ok {
				_ = s.WriteLiteral("1\n")
			} else {
				_ = s.WriteLiteral("0\n")
			}
		}
		return nil
	})
}
