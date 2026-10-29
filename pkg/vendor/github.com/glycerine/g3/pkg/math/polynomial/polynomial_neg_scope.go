package polynomial

// Source: src/math/polynomial/polynomial.cpp:5940-5951;
// src/util/scoped_numeral.h:31-33.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Embedded in the actual core: these are the existing Neg NUM and result,
// together with exact owning continuation metadata for that lexical scope.

type polynomialNegScope struct {
	negativeCoefficient                     unumeral.Int
	negativeResult                          Polynomial
	negativeActive, negativeEnded           bool
	negativeExecuting                       bool
	normalize                               unumeral.IntegerNormalizeInvocationReceipt
	transport                               unumeral.Int
	transportBuffer                         *polynomialSOM
	transportSlot                           uint32
	transportBound                          bool
	receipt                                 polynomialNegInvocationReceipt
	negAppend                               polynomialCheapAppendInvocationReceipt
	negFactory                              polynomialFactoryInvocationReceipt
	internalAppend, internalFactory         bool
	negativeBodyCause, negativeCleanupCause error
	invocation                              uint64
	numeric                                 polynomialNumericRoots
}
type polynomialNegInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialNegScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesNegInvocation(r *polynomialNegInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.polynomialNegScope && r.invocation != 0 && r.invocation == r.frame.invocation && r.frame.negativeActive
}
func (m *MultivariatePolynomialCore) negInvocationSourceCause(r *polynomialNegInvocationReceipt) error {
	if m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.polynomialNegScope && r.invocation != 0 && r.invocation == r.frame.invocation {
		return r.frame.negativeCleanupCause
	}
	return nil
}
func (m *MultivariatePolynomialCore) finishNegSource() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	s := &m.polynomialNegScope
	if s.negativeCleanupCause != nil {
		return s.negativeCleanupCause
	}
	if !s.negativeActive || s.negativeEnded {
		return nil
	}

	_, e := upolynomialSourceNumeralDel(m.integer, &s.negativeCoefficient)
	if e != nil {
		s.negativeCleanupCause = e
		return e
	}
	s.negativeEnded = true
	// A returned factory is exposed to the caller only after this source DT.
	s.negativeResult = Polynomial{}
	if !s.directPending(m) {
		s.negativeActive = false
	}
	return nil
}

func (m *MultivariatePolynomialCore) cleanupNegInvocation(r *polynomialNegInvocationReceipt, abort bool) error {
	if !m.matchesNegInvocation(r) {
		return nil
	}
	return m.cleanupNegPhase(abort)
}
func (m *MultivariatePolynomialCore) cleanupNegPhase(abort bool) error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	s := &m.polynomialNegScope
	if !s.negativeActive {
		return nil
	}
	if s.negativeExecuting {
		return ErrPolynomialPending
	}
	if s.internalAppend {
		if e := m.cheap.cleanupAppendInvocation(&s.negAppend, false); e != nil {
			return e
		}
	}
	if e := m.modular.CleanupPNormalizeInvocation(&s.normalize, false); e != nil {
		return e
	}
	if !s.negativeEnded {
		if e := m.deleteNumeral(&s.negativeCoefficient); e != nil {
			return e
		}
		s.negativeEnded = true
	}
	if abort {
		if s.transportBound {
			if e := m.deleteNumeral(&s.transport); e != nil {
				return e
			}
			s.transportBound = false
			s.transportBuffer = nil
			s.transportSlot = 0
		}
		if e := m.modular.CleanupPNormalizeInvocation(&s.normalize, true); e != nil {
			return e
		}
		if s.negativeResult != (Polynomial{}) {
			if e := m.CleanupValue(&s.negativeResult); e != nil {
				return e
			}
		}
		if s.internalFactory {
			if e := m.cleanupFactoryInvocation(&s.negFactory, true); e != nil {
				return e
			}
		}
		if s.internalAppend {
			if e := m.cheap.cleanupAppendInvocation(&s.negAppend, true); e != nil {
				return e
			}
		}
		if e := s.numeric.cleanup(m.integer); e != nil {
			return e
		}
	}
	if s.negativeResult == (Polynomial{}) && !s.directPending(m) {
		invocation := s.invocation
		*s = polynomialNegScope{invocation: invocation, negativeBodyCause: s.negativeBodyCause, negativeCleanupCause: s.negativeCleanupCause, normalize: s.normalize}
	}
	return nil
}

// This is the existing source entry, including the cheap-buffer assertion and
// the actual minus_a NUM construction. No polynomial operand is read here.
func (m *MultivariatePolynomialCore) beginNegInvocation(out *polynomialNegInvocationReceipt) (*polynomialNegInvocationReceipt, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	if e := polynomialArithmeticSourceAssert(5941, func() (bool, error) {
		n, e := m.cheap.ms.size(m)
		return n == 0, e
	}); e != nil {
		return nil, e
	}
	if m.negativeActive || m.invocation == ^uint64(0) {
		return nil, ErrPolynomialPending
	}
	invocation := m.invocation + 1
	m.polynomialNegScope = polynomialNegScope{invocation: invocation, negativeActive: true, negativeExecuting: true}
	s := &m.polynomialNegScope
	s.receipt = polynomialNegInvocationReceipt{manager: m, frame: s, invocation: invocation}
	if out != nil {
		*out = s.receipt
	}
	return &s.receipt, nil
}

func (s *polynomialNegScope) directPending(m *MultivariatePolynomialCore) bool {
	return s.transportBound || polynomialNegNumericPending(&s.numeric) || m.modular.MatchesPNormalizeInvocation(&s.normalize) || (s.internalAppend && m.cheap.matchesAppendInvocation(&s.negAppend)) || (s.internalFactory && m.matchesFactoryInvocation(&s.negFactory))
}
