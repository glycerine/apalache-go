package polynomial

// Source: src/math/polynomial/polynomial.cpp:4909,4955-4962,4973,5011-5013,5072-5077.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
)

type polynomialPseudoTraceBorrow struct{ executing bool }

func (b *polynomialPseudoTraceBorrow) NumericExecuting() bool { return b.executing }
func (m *MultivariatePolynomialCore) emitPseudoTrace(line int, body func(*polynomialTraceSink) error) error {
	return m.emitPseudoConditionalTrace("polynomial", "pseudo_division_core", line, nil, body)
}
func (m *MultivariatePolynomialCore) emitPseudoConditionalTrace(tag, function string, line int, predicate func() (bool, error), body func(*polynomialTraceSink) error) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	borrow := polynomialPseudoTraceBorrow{executing: true}
	var integer, natural unumeral.NumericCallerBinding
	if e := m.integer.BindCaller(&integer, &borrow, 0); e != nil {
		return e
	}
	defer func() {
		if e := integer.Unbind(); e != nil {
			panic(e)
		}
	}()
	if e := m.integer.Natural.BindCaller(&natural, &borrow, 0); e != nil {
		return e
	}
	defer func() {
		if e := natural.Unbind(); e != nil {
			panic(e)
		}
	}()
	previous := m.quadraticTrace
	m.quadraticTrace = &polynomialQuadraticTraceBorrow{tags: tags}
	defer func() { m.quadraticTrace = previous }()
	var computation error
	_ = tags.EmitConditional(tag, function, "src/math/polynomial/polynomial.cpp", line, func() bool {
		if predicate == nil {
			return true
		}
		ok, e := predicate()
		computation = e
		return e == nil && ok
	}, func(w io.Writer) error { computation = body(&polynomialTraceSink{writer: w}); return computation })
	return computation
}
func (m *MultivariatePolynomialCore) pseudoTracePolynomial(out *polynomialTraceSink, label string, p Polynomial, direct bool) error {
	_ = out.WriteLiteral(label)
	r := &m.pseudoDivision.traceDisplay
	if m.displayReceiptPending(r) {
		return ErrPolynomialPending
	}
	*r = polynomialDisplayInvocationReceipt{}
	if direct {
		return m.displayPolynomialDirectWithReceipt(p, out, false, r)
	}
	return m.displayIntoCoreWithReceipt(p, out, false, r)
}
func (m *MultivariatePolynomialCore) tracePseudoEntry(p, q Polynomial, x uint32) error {
	return m.emitPseudoTrace(4909, func(out *polynomialTraceSink) error {
		if e := m.pseudoTracePolynomial(out, "pseudo_division\np: ", p, true); e != nil {
			return e
		}
		if e := m.pseudoTracePolynomial(out, "\nq: ", q, true); e != nil {
			return e
		}
		_ = out.WriteLiteral("\nx: ")
		_ = out.WriteUnsigned(x)
		return out.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialCore) tracePseudoIteration(p, q Polynomial, Q, R *polynomialReference, d *uint32) error {
	return m.emitPseudoTrace(4955, func(out *polynomialTraceSink) error {
		s := &m.pseudoDivision
		if e := m.pseudoTracePolynomial(out, "A: ", p, true); e != nil {
			return e
		}
		if e := m.pseudoTracePolynomial(out, "\nB: ", q, true); e != nil {
			return e
		}
		if e := m.pseudoTraceReference(out, "\nl_B: ", &s.refs[pseudoLeadB], true); e != nil {
			return e
		}
		if e := m.pseudoTraceReference(out, "\nr_B: ", &s.refs[pseudoReductB], true); e != nil {
			return e
		}
		if e := m.pseudoTraceReference(out, "\nQ: ", Q, true); e != nil {
			return e
		}
		if e := m.pseudoTraceReference(out, "\nR: ", R, true); e != nil {
			return e
		}
		_ = out.WriteLiteral("\nd: ")
		_ = out.WriteUnsigned(*d)
		return out.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialCore) tracePseudoAdjusted() error {
	return m.emitPseudoTrace(4973, func(out *polynomialTraceSink) error {
		if e := m.pseudoTraceReference(out, "l_B_e: ", &m.pseudoDivision.refs[pseudoAdjusted], false); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}

func (m *MultivariatePolynomialCore) pseudoTraceReference(out *polynomialTraceSink, label string, r *polynomialReference, direct bool) error {
	_ = out.WriteLiteral(label)
	return m.pseudoTracePolynomial(out, "", r.value, direct)
}

func (m *MultivariatePolynomialCore) tracePseudoReducedMonomial(term Monomial, x, degR, degB uint32) error {
	return m.emitPseudoConditionalTrace("polynomial", "pseudo_division_core", 5011, func() (bool, error) {
		degree, e := m.monomials.DegreeOf(term, x)
		return degree != degR-degB, e
	}, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral("deg_R: ")
		_ = out.WriteUnsigned(degR)
		_ = out.WriteLiteral(", deg_B: ")
		_ = out.WriteUnsigned(degB)
		_ = out.WriteLiteral(", x: ")
		_ = out.WriteUnsigned(x)
		_ = out.WriteLiteral("\n")
		if e := m.monomials.DisplayInto(term, out, false); e != nil {
			return e
		}
		_ = out.WriteLiteral(", ")
		if e := m.monomials.DisplayInto(m.pseudoDivision.monomial.value, out, false); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialCore) tracePseudoRemainderVerification(R *polynomialReference) error {
	return m.emitPseudoConditionalTrace("pseudo_remainder", "pseudo_remainder", 5072, nil, func(out *polynomialTraceSink) error {
		s := &m.pseudoDivision
		if e := m.pseudoTraceReference(out, "pseudo_division bug\np:   ", &s.refs[pseudoOldP], true); e != nil {
			return e
		}
		if e := m.pseudoTraceReference(out, "\nq:   ", &s.refs[pseudoOldQ], true); e != nil {
			return e
		}
		if e := m.pseudoTraceReference(out, "\nQ:   ", &s.refs[pseudoQuotient], false); e != nil {
			return e
		}
		if e := m.pseudoTraceReference(out, "\nR:   ", R, false); e != nil {
			return e
		}
		if e := m.pseudoTraceReference(out, "\nl^d: ", &s.refs[pseudoPowered], false); e != nil {
			return e
		}
		if e := m.pseudoTraceReference(out, "\nlhs: ", &s.refs[pseudoLeft], false); e != nil {
			return e
		}
		if e := m.pseudoTraceReference(out, "\nrhs: ", &s.refs[pseudoRight], false); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
