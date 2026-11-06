package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1648-1667,2929-2931.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// The scoped inverse belongs to the division invocation, including when a
// nested multiplication retains borrowed inputs pending owning cleanup.
type algebraicDivisionScope struct {
	polynomialCaller          *polynomialAlgebraicEvaluationScope
	sourceStop                error
	inverse                   Algebraic
	active, running, cleaning bool
}

func (m *AlgebraicRootCore) cleanupDivision() error {
	return m.finishDivision(true)
}

func (m *AlgebraicRootCore) finishDivision(owning bool) error {
	s := &m.division
	if s.running || s.cleaning {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	s.cleaning = true
	defer func() { s.cleaning = false }()
	// Only this invocation's private scoped value is destroyed here. Public
	// admission remains blocked by younger unjoined work and during callbacks.
	l, e := m.locationCleanup(&s.inverse)
	if e != nil {
		return e
	}
	if l != nil {
		if e = m.destroyCell(&s.inverse, l, owning); e != nil {
			return e
		}
	}
	*s = algebraicDivisionScope{sourceStop: s.sourceStop, cleaning: true}
	return nil
}

// Div copies the divisor before inversion, so either input may alias out.
// CleanupRetained owns the scoped copy if a nested operation stops.
func (m *AlgebraicRootCore) Div(a, b, out *Algebraic) (cause error) {
	if e := m.check(); e != nil {
		return e
	}
	s := &m.division
	if s.active {
		return ErrAlgebraicPending
	}
	l, e := m.location(b)
	if e != nil {
		return e
	}
	if l == nil {
		if e := diagnostic.BorrowProcessDebugContext().UnreachableSource("src/math/polynomial/algebraic_numbers.cpp", 1650); e != nil {
			return e
		}
		return ErrAlgebraicDivisionByZero
	}
	*s = algebraicDivisionScope{active: true, running: true, polynomialCaller: m.polynomialCaller}
	defer func() {
		s.running = false
		if stop := m.rootTraceSourceCause(); stop != nil {
			s.sourceStop = stop
			return
		}
		// The binary selector borrows the inverse until its local scope ends.
		if !m.binary.active && m.inversion.remaining == 0 {
			cleanup := m.finishDivision(false)
			if cleanup != nil {
				s.sourceStop = cleanup
			}
			cause = errors.Join(cause, cleanup)
		}
	}()
	if e = m.Copy(&s.inverse, b); e != nil {
		return e
	}
	if e = m.Inv(&s.inverse); e != nil {
		return e
	}
	return m.Mul(a, &s.inverse, out)
}
