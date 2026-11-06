package polynomial

// Source: src/math/polynomial/polynomial.cpp:6595,6731.

import "errors"

// The leaf helpers have no source locals. An enabled display nevertheless
// needs its own accepted invocation, before the factor push acquires p.
func (m *MultivariatePolynomialManager) traceFactorLeaf(p Polynomial, target *PolynomialFactors, function string, line int, label string, receipt *polynomialGCDInvocationReceipt) (err error) {
	if !m.core.monomials.monomialTraceTags().ShouldTrace("factor") {
		return nil
	}
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	if receipt != nil {
		*receipt = polynomialGCDInvocationReceipt{m, s, s.invocation}
	}
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		m.core.pscLeave()
	}()
	return m.emitFactorTrace(target, function, line, func(sink *polynomialTraceSink) error {
		if e := m.quadraticTracePolynomial(sink, s, label, p, true); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}
