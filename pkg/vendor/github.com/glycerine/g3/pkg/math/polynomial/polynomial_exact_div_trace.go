package polynomial

// Source: src/math/polynomial/polynomial.cpp:5117,5143-5144.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
)

// Diagnostic locals precede the zero shortcut and the algorithm's locals.
// Their actual accepted display has a separate, generation-bound receipt.
type polynomialExactDivTraceScope struct {
	tags       *diagnostic.TraceTags
	invocation uint64
	running    bool
	current    polynomialExactDivTraceInvocation
	display    polynomialDisplayInvocationReceipt
}
type polynomialExactDivTraceInvocation struct {
	core       *MultivariatePolynomialCore
	frame      *polynomialExactDivTraceScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesExactDivTrace(r *polynomialExactDivTraceInvocation) bool {
	return m != nil && m.self == m && r != nil && r.core == m && r.frame == &m.exactDivTrace && r.invocation != 0 && r.invocation == m.exactDivTrace.invocation
}
func (m *MultivariatePolynomialCore) exactDivTraceInvocationPending(r *polynomialExactDivTraceInvocation) bool {
	return m.matchesExactDivTrace(r) && m.displayReceiptPending(&m.exactDivTrace.display)
}
func (m *MultivariatePolynomialCore) exactDivTraceSourceCause(r *polynomialExactDivTraceInvocation) error {
	if !m.matchesExactDivTrace(r) {
		return nil
	}
	return m.displayReceiptSourceCause(&m.exactDivTrace.display)
}
func (m *MultivariatePolynomialCore) cleanupExactDivTrace(r *polynomialExactDivTraceInvocation) error {
	if !m.matchesExactDivTrace(r) {
		return nil
	}
	if m.exactDivTrace.running {
		return ErrPolynomialPending
	}
	return m.cleanupDisplayReceipt(&m.exactDivTrace.display)
}
func (m *MultivariatePolynomialCore) acceptExactDivTrace(r *polynomialExactDivTraceInvocation) error {
	s := &m.exactDivTrace
	if s.running || m.displayReceiptPending(&s.display) {
		return ErrPolynomialPending
	}
	if m.matchesExactDivTrace(r) {
		return nil
	}
	if s.invocation == ^uint64(0) {
		return ErrPolynomialPending
	}
	*s = polynomialExactDivTraceScope{invocation: s.invocation + 1}
	s.current = polynomialExactDivTraceInvocation{m, s, s.invocation}
	*r = s.current
	return nil
}
func (m *MultivariatePolynomialCore) emitExactDivTrace(tag string, line int, r *polynomialExactDivTraceInvocation, body func(*polynomialTraceSink) error) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	if e := m.acceptExactDivTrace(r); e != nil {
		return e
	}
	s := &m.exactDivTrace
	s.running = true
	s.tags = tags
	previousMonomial := m.monomials.exactDivTrace
	m.monomials.exactDivTrace = s
	defer func() { m.monomials.exactDivTrace = previousMonomial }()
	var integer, natural unumeral.NumericCallerBinding
	if e := m.integer.BindCaller(&integer, s, 0); e != nil {
		s.running = false
		return e
	}
	defer func() {
		if e := integer.Unbind(); e != nil {
			panic(e)
		}
	}()
	if e := m.integer.Natural.BindCaller(&natural, s, 0); e != nil {
		s.running = false
		return e
	}
	defer func() {
		if e := natural.Unbind(); e != nil {
			panic(e)
		}
	}()
	defer func() { s.running = false }()
	previous := m.quadraticTrace
	m.quadraticTrace = &polynomialQuadraticTraceBorrow{tags: tags}
	defer func() { m.quadraticTrace = previous }()
	var computation error
	_ = tags.EmitConditional(tag, "exact_div", "src/math/polynomial/polynomial.cpp", line, func() bool { return true }, func(w io.Writer) error {
		computation = body(&polynomialTraceSink{writer: w})
		return computation
	})
	return computation
}
func (m *MultivariatePolynomialCore) traceExactDivEntry(p, q Polynomial, r *polynomialExactDivTraceInvocation) error {
	return m.emitExactDivTrace("polynomial", 5117, r, func(sink *polynomialTraceSink) error {
		_ = sink.WriteLiteral("exact division\np: ")
		s := &m.exactDivTrace
		s.display = polynomialDisplayInvocationReceipt{}
		if e := m.displayPolynomialDirectWithReceipt(p, sink, false, &s.display); e != nil {
			return e
		}
		_ = sink.WriteLiteral("\nq: ")
		s.display = polynomialDisplayInvocationReceipt{}
		if e := m.displayPolynomialDirectWithReceipt(q, sink, false, &s.display); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialCore) traceExactDivMonomials(mr, mq, quotient Monomial, r *polynomialExactDivTraceInvocation) error {
	return m.emitExactDivTrace("polynomial", 5143, r, func(sink *polynomialTraceSink) error {
		_ = sink.WriteLiteral("m_r: ")
		if e := m.monomials.DisplayInto(mr, sink, false); e != nil {
			return e
		}
		_ = sink.WriteLiteral("\nm_q: ")
		if e := m.monomials.DisplayInto(mq, sink, false); e != nil {
			return e
		}
		_ = sink.WriteLiteral("\n")
		if quotient != (Monomial{}) {
			_ = sink.WriteLiteral("m_r_q: ")
			if e := m.monomials.DisplayInto(quotient, sink, false); e != nil {
				return e
			}
			_ = sink.WriteLiteral("\n")
		}
		return nil
	})
}

func (s *polynomialExactDivTraceScope) NumericExecuting() bool { return s.running }
func (m *MonomialManager) exactDivTraceCallback() bool {
	return m != nil && m.self == m && m.exactDivTrace != nil && m.exactDivTrace.running && (m.exactDivTrace.tags.OutputCallbackActive() || m.memory.FailureTransportActive())
}
