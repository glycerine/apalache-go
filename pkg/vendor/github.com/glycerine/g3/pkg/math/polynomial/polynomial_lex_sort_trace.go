package polynomial

// Source: src/math/polynomial/polynomial.cpp:1450-1454.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
)

type polynomialLexTraceBorrow struct {
	executing bool
	tags      *diagnostic.TraceTags
}

func (b *polynomialLexTraceBorrow) NumericExecuting() bool { return b.executing }
func (m *MonomialManager) lexTraceCallback() bool {
	return m != nil && m.self == m && m.lexTraceBorrow != nil && m.lexTraceBorrow.executing && (m.lexTraceBorrow.tags.OutputCallbackActive() || m.memory.FailureTransportActive())
}
func (m *MultivariatePolynomialManager) lexTracePending() bool {
	return m != nil && m.self == m && m.core != nil && m.lexTraceGeneration != 0 && m.lexTraceGeneration == m.lexGeneration && m.core.displayReceiptPending(&m.lexTraceDisplay)
}
func (m *MultivariatePolynomialCore) pendingLexSortCause() error {
	if m == nil || m.self != m || m.pendingLexSort == nil || m.pendingLexSort.core != m || !m.pendingLexSort.lexTracePending() {
		return nil
	}
	if e := m.displayReceiptSourceCause(&m.pendingLexSort.lexTraceDisplay); e != nil {
		return e
	}
	return ErrPolynomialPending
}

// Each macro evaluates its own pair; no pointer or comparison is shared across
// the conditional record and the independently gated source assertion.
func (m *MultivariatePolynomialManager) lexAdjacentOrder(p Polynomial, i uint32) (int, error) {
	ms, e := m.core.captureMonomialPointer(p, 0)
	if e != nil {
		return 0, e
	}
	a, e := ms.read(i)
	if e != nil {
		return 0, e
	}
	b, e := ms.read(i + 1)
	if e != nil {
		return 0, e
	}
	return m.core.monomials.Lex(a, b)
}
func (m *MultivariatePolynomialManager) lexDebugCheck(p Polynomial) error {
	for i := uint32(0); ; i++ {
		n, e := m.core.readWord(p, 8)
		if e != nil {
			return e
		}
		if i >= n-1 {
			return nil
		}
		if e = m.traceLexAdjacent(p, i); e != nil {
			return e
		}
		if e = polynomialLexSortSourceAssert(1453, "lex_compare(m_ms[i], m_ms[i+1]) > 0", func() (bool, error) { order, e := m.lexAdjacentOrder(p, i); return order > 0, e }); e != nil {
			return e
		}
	}
}
func (m *MultivariatePolynomialManager) traceLexAdjacent(p Polynomial, i uint32) error {
	tags := m.core.monomials.monomialTraceTags()
	if !tags.ShouldTrace("poly_bug") {
		return nil
	}
	borrow := polynomialLexTraceBorrow{executing: true, tags: tags}
	previousMM := m.core.monomials.lexTraceBorrow
	m.core.monomials.lexTraceBorrow = &borrow
	defer func() { m.core.monomials.lexTraceBorrow = previousMM }()
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
	previous := m.core.quadraticTrace
	m.core.quadraticTrace = &polynomialQuadraticTraceBorrow{tags: tags}
	defer func() { m.core.quadraticTrace = previous }()
	var computation error
	_ = tags.EmitConditional("poly_bug", "lex_sort", "src/math/polynomial/polynomial.cpp", 1451, func() bool {
		order, e := m.lexAdjacentOrder(p, i)
		computation = e
		return e == nil && order <= 0
	}, func(w io.Writer) error {
		out := &polynomialTraceSink{writer: w}
		_ = out.WriteLiteral("i: ")
		_ = out.WriteUnsigned(i)
		_ = out.WriteLiteral("\npoly: ")
		if m.core.displayReceiptPending(&m.lexTraceDisplay) {
			computation = ErrPolynomialPending
			return computation
		}
		m.lexTraceDisplay = polynomialDisplayInvocationReceipt{}
		m.lexTraceGeneration = m.lexGeneration
		computation = m.core.displayPolynomialDirectWithReceipt(p, out, false, &m.lexTraceDisplay)
		if m.lexTracePending() {
			m.core.pendingLexSort = m
		}
		if computation != nil {
			return computation
		}
		return out.WriteLiteral("\n")
	})
	return computation
}

// Accepted only after the actual nontrivial sort generation advances.
type polynomialLexSortInvocationReceipt struct {
	manager    *MultivariatePolynomialManager
	generation uint64
}

func (m *MultivariatePolynomialManager) matchesLexSortInvocation(r *polynomialLexSortInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.generation != 0 && r.generation == m.lexGeneration && m.lexPending
}
func (m *MultivariatePolynomialManager) cleanupLexSortInvocation(r *polynomialLexSortInvocationReceipt) error {
	if !m.matchesLexSortInvocation(r) {
		return nil
	}
	return m.cleanupLexSort()
}
func (m *MultivariatePolynomialManager) lexSortInvocationSourceCause(r *polynomialLexSortInvocationReceipt) error {
	if !m.matchesLexSortInvocation(r) {
		return nil
	}
	return m.core.displayReceiptSourceCause(&m.lexTraceDisplay)
}
