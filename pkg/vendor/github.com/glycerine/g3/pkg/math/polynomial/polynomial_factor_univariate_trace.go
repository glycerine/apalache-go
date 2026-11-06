package polynomial

// Source: src/math/polynomial/polynomial.cpp:6707-6720.

func (m *MultivariatePolynomialManager) emitUnivariateFactorBug(s *polynomialFactorUnivariateScope, line int, body func(*polynomialTraceSink) error) error {
	tags := m.core.monomials.monomialTraceTags()
	if !tags.ShouldTrace("factor_bug") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m.upm}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	return m.emitFactorTraceTag(s.target, "factor_bug", "factor_sqf_pp_univ", line, body)
}
func (m *MultivariatePolynomialManager) traceUnivariateFactorStart(s *polynomialFactorUnivariateScope, p Polynomial) error {
	return m.emitUnivariateFactorBug(s, 6707, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral("factoring fs constant: ")
		// imp refines numeral_manager to mpzzp_manager (cpp1823), so m()
		// preserves its normalization assertion and to_string call. Retained
		// primitive square-free factors have constant +/-1: no character child.
		text, e := m.core.modular.SourceStringTracked(&s.fs.constant, nil)
		if e != nil {
			return e
		}
		_ = out.WriteLiteral(text)
		_ = out.WriteLiteral("\np:\n")
		if e = m.core.displayPolynomialDirectWithReceipt(p, out, false, &s.tracePolynomial); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceUnivariateFactorConverted(s *polynomialFactorUnivariateScope, vector *upolyVectorState) error {
	return m.emitUnivariateFactorBug(s, 6715, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral("uni-factor:\n")
		if e := m.upm.displayIntoWithReceipt(m.upm.stateView(vector), factorizationTraceStream{out}, "x", false, &s.traceUnivariate); e != nil {
			return e
		}
		_ = out.WriteLiteral("\nfactor:\n")
		if e := m.core.displayIntoCoreWithReceipt(s.f.value, out, false, &s.tracePolynomial); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceUnivariateFactorEnd(s *polynomialFactorUnivariateScope) error {
	return m.emitUnivariateFactorBug(s, 6720, func(out *polynomialTraceSink) error { return out.WriteLiteral("end-factors...\n") })
}
