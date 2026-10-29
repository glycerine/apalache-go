package polynomial

// Source: src/math/polynomial/polynomial.cpp:6604,6612,6636,6652,6666-6672.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type polynomialQuadraticTraceBorrow struct {
	tags   *diagnostic.TraceTags
	target *PolynomialFactors
}

func (m *MultivariatePolynomialCore) quadraticTraceCallback() bool {
	if m == nil || m.self != m || m.quadraticTrace == nil {
		return false
	}
	if m.callbackDepth != 0 || m.quadraticTrace.tags.OutputCallbackActive() {
		return true
	}
	rt := m.integer.RuntimeOwner()
	return rt != nil && rt.FailureTransportActive()
}
func (m *MultivariatePolynomialManager) emitQuadraticTrace(target *PolynomialFactors, line int, body func(*polynomialTraceSink) error) error {
	return m.emitFactorTrace(target, "factor_2_sqf_pp", line, body)
}
func (m *MultivariatePolynomialManager) emitFactorTrace(target *PolynomialFactors, function string, line int, body func(*polynomialTraceSink) error) error {
	return m.emitFactorTraceTag(target, "factor", function, line, body)
}
func (m *MultivariatePolynomialManager) emitFactorTraceTag(target *PolynomialFactors, tag, function string, line int, body func(*polynomialTraceSink) error) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	previous := c.quadraticTrace
	c.quadraticTrace = &polynomialQuadraticTraceBorrow{tags: tags, target: target}
	defer func() { c.quadraticTrace = previous }()
	var computation error
	_ = tags.EmitConditional(tag, function, "src/math/polynomial/polynomial.cpp", line, func() bool { return true }, func(w io.Writer) error {
		computation = body(&polynomialTraceSink{writer: w})
		return computation
	})
	return computation
}
func (m *MultivariatePolynomialManager) quadraticTracePolynomial(sink *polynomialTraceSink, s *polynomialGCDFunctionScope, label string, p Polynomial, direct bool) error {
	_ = sink.WriteLiteral(label)
	if m.core.displayReceiptPending(&s.traceDisplay) {
		return ErrPolynomialPending
	}
	s.traceDisplay = polynomialDisplayInvocationReceipt{}
	if direct {
		return m.core.displayPolynomialDirectWithReceipt(p, sink, false, &s.traceDisplay)
	}
	return m.core.displayIntoCoreWithReceipt(p, sink, false, &s.traceDisplay)
}
func (m *MultivariatePolynomialManager) traceQuadraticEntry(p Polynomial, target *PolynomialFactors, s *polynomialGCDFunctionScope) error {
	return m.emitQuadraticTrace(target, 6604, func(sink *polynomialTraceSink) error {
		if e := m.quadraticTracePolynomial(sink, s, "factor square free (degree == 2):\n", p, true); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceQuadraticCoefficients(target *PolynomialFactors, s *polynomialGCDFunctionScope) error {
	return m.emitQuadraticTrace(target, 6612, func(sink *polynomialTraceSink) error {
		for i, label := range [...]string{"a: ", "\nb: ", "\nc: "} {
			if e := m.quadraticTracePolynomial(sink, s, label, s.refs[i].root.value, false); e != nil {
				return e
			}
		}
		return sink.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceQuadraticReference(target *PolynomialFactors, s *polynomialGCDFunctionScope, line int, label string, p Polynomial) error {
	return m.emitQuadraticTrace(target, line, func(sink *polynomialTraceSink) error {
		if e := m.quadraticTracePolynomial(sink, s, label, p, false); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceQuadraticBeforePrimitive(target *PolynomialFactors, s *polynomialGCDFunctionScope, x uint32) error {
	return m.emitQuadraticTrace(target, 6666, func(sink *polynomialTraceSink) (err error) {
		if e := m.quadraticTracePolynomial(sink, s, "before pp\nf1: ", s.refs[7].root.value, false); e != nil {
			return e
		}
		if e := m.quadraticTracePolynomial(sink, s, "\nf2: ", s.refs[8].root.value, false); e != nil {
			return e
		}
		_ = sink.WriteLiteral("\n")
		frontier := s.depth
		defer func() { err = errors.Join(err, m.finishGCDLocalsTo(s, frontier)) }()
		s.constructReference(m.core, 12)
		if e := m.Content(s.refs[7].root.value, x, &s.refs[12]); e != nil {
			return e
		}
		s.constructReference(m.core, 13)
		if e := m.Content(s.refs[8].root.value, x, &s.refs[13]); e != nil {
			return e
		}
		if e := m.quadraticTracePolynomial(sink, s, "content(f1): ", s.refs[12].root.value, false); e != nil {
			return e
		}
		if e := m.quadraticTracePolynomial(sink, s, "\ncontent(f2): ", s.refs[13].root.value, false); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceQuadraticFactors(target *PolynomialFactors, s *polynomialGCDFunctionScope) error {
	return m.emitQuadraticTrace(target, 6672, func(sink *polynomialTraceSink) error {
		if e := m.quadraticTracePolynomial(sink, s, "f1: ", s.refs[7].root.value, false); e != nil {
			return e
		}
		if e := m.quadraticTracePolynomial(sink, s, "\nf2: ", s.refs[8].root.value, false); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}
