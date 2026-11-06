package polynomial

// Source: src/math/polynomial/polynomial.cpp:6795,6807.

func (m *MultivariatePolynomialManager) traceFactorCoreLoop(s *polynomialFactorCoreFrame, j uint32) error {
	return m.emitFactorTrace(s.target, "factor_core", 6795, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral("factor_core main loop j: ")
		_ = out.WriteUnsigned(j)
		_ = out.WriteLiteral("\nA: ")
		if e := m.core.displayIntoCoreWithReceipt(s.refs[5].value, out, false, &s.traceDisplay); e != nil {
			return e
		}
		_ = out.WriteLiteral("\nB: ")
		if e := m.core.displayIntoCoreWithReceipt(s.refs[4].value, out, false, &s.traceDisplay); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceFactorCoreConstant(s *polynomialFactorCoreFrame) error {
	return m.emitFactorTrace(s.target, "factor_core", 6807, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral("const C: ")
		if e := m.core.displayIntoCoreWithReceipt(s.refs[2].value, out, false, &s.traceDisplay); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}

// Source cpp6769: scoped_numeral's stream operator calls raw integer display.
func (m *MultivariatePolynomialManager) traceFactorCoreInteger(s *polynomialFactorCoreFrame) error {
	return m.emitFactorTrace(s.target, "factor_core", 6769, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral("i: ")
		if e := m.core.integer.DisplayToTracked(out, &s.numeral, &s.traceNumeral); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
