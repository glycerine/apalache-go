package polynomial

// Source: src/math/polynomial/polynomial.cpp:5362,5402-5403,5424.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

type polynomialResultantTraceBorrow struct{ executing bool }

func (b *polynomialResultantTraceBorrow) NumericExecuting() bool { return b.executing }

func (m *MultivariatePolynomialManager) emitResultantTrace(function string, line int, body func(*polynomialTraceSink) error) error {
	if !m.core.monomials.monomialTraceTags().ShouldTrace("resultant") {
		return nil
	}
	borrow := polynomialResultantTraceBorrow{executing: true}
	var integer, natural unumeral.NumericCallerBinding
	if e := m.core.integer.BindCaller(&integer, &borrow, 0); e != nil {
		return e
	}
	defer func() {
		if e := integer.Unbind(); e != nil {
			panic(e)
		}
	}()
	if e := m.core.integer.Natural.BindCaller(&natural, &borrow, 0); e != nil {
		return e
	}
	defer func() {
		if e := natural.Unbind(); e != nil {
			panic(e)
		}
	}()
	return m.emitFactorTraceTag(nil, "resultant", function, line, body)
}

func (m *MultivariatePolynomialManager) traceResultant(s *polynomialGCDFunctionScope, x uint32, normalized bool) error {
	line, prefix := 5362, "resultant(A, B, x)\nA: "
	if normalized {
		line, prefix = 5402, "resultant(A, B, x) after normalization\nA: "
	}
	return m.emitResultantTrace("resultant", line, func(out *polynomialTraceSink) error {
		if e := m.quadraticTracePolynomial(out, s, prefix, s.refs[0].root.value, false); e != nil {
			return e
		}
		if e := m.quadraticTracePolynomial(out, s, "\nB: ", s.refs[1].root.value, false); e != nil {
			return e
		}
		_ = out.WriteLiteral("\nx: ")
		_ = out.WriteUnsigned(x)
		if normalized {
			if e := m.quadraticTracePolynomial(out, s, "\nt: ", s.refs[6].root.value, false); e != nil {
				return e
			}
		}
		return out.WriteLiteral("\n")
	})
}

func (m *MultivariatePolynomialManager) traceResultantIteration(s *polynomialGCDFunctionScope) error {
	return m.emitResultantTrace("resultant", 5424, func(out *polynomialTraceSink) error {
		if e := m.quadraticTracePolynomial(out, s, "A: ", s.refs[0].root.value, false); e != nil {
			return e
		}
		if e := m.quadraticTracePolynomial(out, s, "\nB: ", s.refs[1].root.value, false); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
