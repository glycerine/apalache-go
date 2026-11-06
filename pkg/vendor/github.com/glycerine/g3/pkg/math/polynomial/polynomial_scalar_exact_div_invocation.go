package polynomial

// Source: src/math/polynomial/polynomial.cpp:5094-5114.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The scalar overload constructs one RAW numeral after resetting R. This
// receipt names that actual declaration, not a snapshot of integer scratch.

type polynomialScalarExactDivInvocationReceipt struct {
	prelude    polynomialArithmeticInvocationReceipt
	core       *MultivariatePolynomialCore
	frame      *polynomialExactDivisionScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesScalarExactDivHistory(r *polynomialScalarExactDivInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.core == m && r.frame == &m.exactDivision && r.invocation != 0 && r.frame.invocation == r.invocation && r.frame.kind == 2
}
func (m *MultivariatePolynomialCore) matchesScalarExactDivInvocation(r *polynomialScalarExactDivInvocationReceipt) bool {
	return m.matchesScalarExactDivHistory(r) && r.frame.active
}
func (m *MultivariatePolynomialCore) scalarExactDivInvocationSourceCause(r *polynomialScalarExactDivInvocationReceipt) error {
	if !m.matchesScalarExactDivHistory(r) {
		return nil
	}
	return r.frame.sourceCause
}

func (m *MultivariatePolynomialCore) scalarExactChildSourceCause(r *polynomialScalarExactDivInvocationReceipt) error {
	if !m.matchesScalarExactDivHistory(r) {
		return nil
	}
	if e := m.arithmeticInvocationSourceCause(&r.prelude); e != nil {
		return e
	}
	return numericIntegerManager_scalarExactNumericSourceCause(m.integer, r)
}
func (m *MultivariatePolynomialCore) cleanupScalarExactDivInvocation(r *polynomialScalarExactDivInvocationReceipt, abort bool) error {
	if !m.matchesScalarExactDivInvocation(r) {
		return nil
	}
	s := r.frame
	if s.executing {
		return ErrPolynomialPending
	}
	// Reached younger source destructors precede the older RAW tmp. Unjoined
	// acquisitions are aborted only after the owning continuation deletes tmp.
	if e := m.cleanupArithmeticInvocation(&r.prelude, false); e != nil {
		return e
	}
	if e := numericIntegerManager_cleanupScalarExactDivNumeric(m.integer, r, false); e != nil {
		return e
	}
	if !abort {
		return nil
	}
	if !s.ownStarted {
		s.ownStarted = true
		s.ownStage = s.stage
	}
	if s.ownStage == 0 {
		if e := m.deleteNumeral(&s.numeral); e != nil {
			return e
		}
		s.ownStage = 1
	}
	if e := numericIntegerManager_cleanupScalarExactDivNumeric(m.integer, r, true); e != nil {
		return e
	}
	if e := m.cleanupArithmeticInvocation(&r.prelude, true); e != nil {
		return e
	}
	if s.returned != (Polynomial{}) {
		if e := m.CleanupValue(&s.returned); e != nil {
			return e
		}
	}
	s.active = false
	s.pscOwner = 0
	return nil
}

func (m *MultivariatePolynomialCore) ExactDivScalar(p Polynomial, c *unumeral.Int) (result Polynomial, err error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if e := polynomialExactDivSourceAssert(5095, "!m().is_zero(c)", func() (bool, error) {
		zero, e := m.modular.IsZero(c)
		return !zero, e
	}); e != nil {
		return Polynomial{}, e
	}
	if m.exactDivision.active {
		return Polynomial{}, ErrPolynomialPending
	}
	if e := m.som.reset(); e != nil {
		return Polynomial{}, e
	}
	s := &m.exactDivision
	if s.invocation == ^uint64(0) {
		return Polynomial{}, ErrPolynomialPending
	}
	var prelude polynomialArithmeticInvocationReceipt
	if e := m.som.acquireArithmeticInvocation(&prelude); e != nil {
		return Polynomial{}, e
	}
	*s = polynomialExactDivisionScope{active: true, executing: true, kind: 2, invocation: s.invocation + 1, pscOwner: m.pscExecutingOwner()}
	s.scalarReceipt = polynomialScalarExactDivInvocationReceipt{prelude: prelude, core: m, frame: s, invocation: s.invocation}
	r := &s.scalarReceipt
	defer func() {
		s.executing = false
		s.bodyCause = err
		if s.sourceCause == nil {
			s.sourceCause = m.scalarExactChildSourceCause(r)
		}
		// An inline failed declaration with no actual child acquisition owns
		// nothing. Empty R can acknowledge its exact metadata without replaying
		// the source RAW DEL that the failed body never reached.
		if err != nil && s.numeral.NativeCellToken() == 0 && !polynomialNegNumericPending(&s.numeric) && !m.scalarExactDirectPending(s) && s.sourceCause == nil {
			m.som.acknowledgeArithmeticSourceReset(&r.prelude)
			if !m.matchesArithmeticInvocation(&r.prelude) {
				s.active = false
				s.pscOwner = 0
			}
		}
		// A successful factory return transfers all R fields to the result. The
		// completed tmp was deleted before that factory; only metadata remains.
		if err == nil {
			if e := m.cleanupArithmeticInvocation(&r.prelude, true); e != nil {
				s.cleanupCause = e
				s.returned = result
				result = Polynomial{}
				err = e
			} else {
				s.active = false
				s.pscOwner = 0
			}
		}
	}()
	n, e := m.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	for i := uint32(0); i < n; i++ {
		a, e := m.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = polynomialExactDivSourceAssert(5104, "m().divides(c, p->a(i))", func() (bool, error) {
			divisible, e := numericIntegerModularManager_dividesWithScalarExactDivReceipt(m.modular, c, &a, r)
			return divisible, e
		}); e != nil {
			return Polynomial{}, e
		}
		if e = numericIntegerModularManager_divWithScalarExactDivReceipt(m.modular, &a, c, &s.numeral, r); e != nil {
			return Polynomial{}, e
		}
		zero, e := m.modular.IsZero(&s.numeral)
		if e != nil {
			return Polynomial{}, e
		}
		if !zero {
			term, e := m.monomialCore(p, i)
			if e != nil {
				return Polynomial{}, e
			}
			if e = (m.som).addWithArithmeticReceipt(&s.numeral, term, &r.prelude); e != nil {
				return Polynomial{}, e
			}
		}
	}
	s.endEntered = true
	if e = m.scalarExactChildSourceCause(r); e != nil {
		s.sourceCause = e
		return Polynomial{}, e
	}
	if _, e = upolynomialSourceNumeralDel(m.integer, &s.numeral); e != nil {
		s.sourceCause = e
		return Polynomial{}, e
	}
	s.stage = 1
	return m.som.mkWithArithmeticReceipt(false, &r.prelude)
}
