package polynomial

// Source: src/math/polynomial/polynomial.cpp:6364-6365,6394-6395.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
)

type polynomialSubstitutionTraceBorrow struct {
	executing bool
	tags      *diagnostic.TraceTags
}

func (b *polynomialSubstitutionTraceBorrow) NumericExecuting() bool { return b.executing }
func (m *MultivariatePolynomialCore) emitIntegerSubstitutionTrace(line int, body func(*polynomialTraceSink) error) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial") {
		return nil
	}
	borrow := polynomialSubstitutionTraceBorrow{executing: true, tags: tags}
	previousMonomial := m.monomials.substitutionTrace
	m.monomials.substitutionTrace = &borrow
	defer func() { m.monomials.substitutionTrace = previousMonomial }()
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
	_ = tags.EmitConditional("polynomial", "substitute", "src/math/polynomial/polynomial.cpp", line, func() bool { return true }, func(w io.Writer) error { computation = body(&polynomialTraceSink{writer: w}); return computation })
	return computation
}
func (m *MultivariatePolynomialCore) traceIntegerSubstitutionEntry(n uint32, xs PolynomialVariablesView, values []unumeral.Int, oneX *uint32, oneValue *unumeral.Int) error {
	return m.emitIntegerSubstitutionTrace(6364, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral("substitute num_vars: ")
		_ = out.WriteUnsigned(n)
		_ = out.WriteLiteral("\n")
		for i := uint32(0); i < n; i++ {
			_ = out.WriteLiteral("x")
			var x uint32
			var e error
			if oneX != nil {
				if i != 0 {
					return arena.ErrBounds
				}
				x = *oneX
			} else {
				x, e = xs.At(i)
				if e != nil {
					return e
				}
			}
			_ = out.WriteUnsigned(x)
			_ = out.WriteLiteral(" -> ")
			var v *unumeral.Int
			if oneX != nil {
				v = oneValue
			} else {
				if uint64(i) >= uint64(len(values)) {
					return arena.ErrBounds
				}
				v = &values[i]
			}
			if v == nil {
				return arena.ErrBounds
			}
			text, e := m.modular.SourceStringTracked(v, &m.substitution.scope.traceInsertion)
			if e != nil {
				return e
			}
			_ = out.WriteLiteral(text)
			_ = out.WriteLiteral("\n")
		}
		return nil
	})
}
func (m *MultivariatePolynomialCore) traceIntegerSubstitutionTerm(p Polynomial, i uint32, term Monomial) error {
	return m.emitIntegerSubstitutionTrace(6394, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral("processing ")
		a, e := m.coefficientCore(p, i)
		if e != nil {
			return e
		}
		text, e := m.modular.SourceStringTracked(&a, &m.substitution.scope.traceInsertion)
		if e != nil {
			return e
		}
		_ = out.WriteLiteral(text)
		_ = out.WriteLiteral(" ")
		if e = m.monomials.displayIntoCore(term, out, false); e != nil {
			return e
		}
		_ = out.WriteLiteral("\nnew_a: ")
		text, e = m.modular.SourceStringTracked(&m.substitution.scope.newA, &m.substitution.scope.traceInsertion)
		if e != nil {
			return e
		}
		_ = out.WriteLiteral(text)
		_ = out.WriteLiteral(" ")
		mono, e := m.generatedSubstitutionMonomial()
		if e != nil {
			return e
		}
		if m.generatedActive && m.generatedMonomial == mono {
			m.substitution.scope.traceMonomial = mono
			m.transferGeneratedMonomial()
			m.substitution.generatedReceipt = Monomial{}
		}
		if e = m.monomials.displayIntoCore(mono, out, false); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}

func (m *MonomialManager) substitutionTraceCallback() bool {
	if m == nil || m.self != m || m.substitutionTrace == nil || !m.substitutionTrace.executing {
		return false
	}
	return m.substitutionTrace.tags.OutputCallbackActive() || m.memory.FailureTransportActive()
}
