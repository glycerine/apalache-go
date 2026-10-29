package polynomial

// Source: src/math/polynomial/polynomial.cpp:5154-5194;
// src/util/obj_ref.h; src/util/scoped_numeral.h.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialDividesScope struct {
	generated                Monomial
	calleeGeneration         uint64
	monomialAllocator        monomialAllocatorInvocationReceipt
	invocation               uint64
	executing                bool
	constructed              uint8
	boolNumeral              unumeral.IntegerDividesNumeralInvocationReceipt
	normalize                unumeral.IntegerNormalizeInvocationReceipt
	divisionRaw, extendedRaw unumeral.IntegerRawInvocationReceipt

	directReceipt           polynomialDividesInvocationReceipt
	monomial                polynomialDivisionReference
	numeral                 unumeral.Int
	numeric                 polynomialNumericRoots
	active                  bool
	stage, ownStage         uint8
	ownStarted              bool
	sourceCause             error
	bodyCause, cleanupCause error
}

func (m *MultivariatePolynomialCore) dividesChildSourceCause(s *polynomialDividesScope) error {
	if e := m.dividesTraceSourceCause(&s.directReceipt.trace); e != nil {
		return e
	}
	if e := m.arithmeticInvocationSourceCause(&s.directReceipt.prelude); e != nil {
		return e
	}
	return numericIntegerManager_dividesNumericSourceCause(m.integer, &s.directReceipt)
}
func (m *MultivariatePolynomialCore) finishDividesSource() error {
	s := &m.dividesLocal
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	if e := m.dividesChildSourceCause(s); e != nil {
		s.sourceCause = e
		return e
	}
	if s.stage == 0 {
		if s.constructed >= 2 {
			if _, e := upolynomialSourceNumeralDel(m.integer, &s.numeral); e != nil {
				s.sourceCause = e
				return e
			}
		}
		s.stage = 1
	}
	if s.stage == 1 {
		if s.constructed >= 1 {
			if e := s.monomial.drop(m, &s.monomial.value, &s.monomial.decrementStarted, false); e != nil {
				s.sourceCause = e
				return e
			}
		}
		s.stage = 2
	}
	if !m.dividesTraceInvocationPending(&s.directReceipt.trace) && !m.matchesArithmeticInvocation(&s.directReceipt.prelude) && !s.monomial.assigning && !numericIntegerManager_dividesNumericPending(m.integer, &s.directReceipt) && s.generated == (Monomial{}) && s.calleeGeneration == 0 {
		s.active = false
	}
	return nil
}
func (m *MultivariatePolynomialCore) CleanupDivides() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if e := m.cleanupDividesTrace(&m.dividesTrace.current); e != nil {
		return e
	}
	return m.cleanupDividesInvocation(&m.dividesLocal.directReceipt, true)
}
func (m *MultivariatePolynomialCore) cleanupDividesPhase(s *polynomialDividesScope, abort bool) error {
	if s.executing {
		return ErrPolynomialPending
	}
	if e := numericIntegerManager_cleanupDividesNumeric(m.integer, &s.directReceipt, false); e != nil {
		return e
	}
	if !s.ownStarted {
		s.ownStage = s.stage
		s.ownStarted = true
	}
	if s.ownStage == 0 {
		if s.constructed >= 2 {
			if e := m.deleteNumeral(&s.numeral); e != nil {
				return e
			}
		}
		s.ownStage = 1
	}
	if s.ownStage == 1 {
		if s.constructed >= 1 {
			if e := s.monomial.drop(m, &s.monomial.value, &s.monomial.decrementStarted, true); e != nil {
				return e
			}
		}
		s.ownStage = 2
	}
	if !abort {
		return nil
	}
	if e := numericIntegerManager_cleanupDividesNumeric(m.integer, &s.directReceipt, true); e != nil {
		return e
	}
	if e := s.monomial.drop(m, &s.monomial.incoming, &s.monomial.incomingStarted, true); e != nil {
		return e
	}
	s.monomial.assigning = false
	if e := m.cleanupDividesCreated(s); e != nil {
		return e
	}
	// The outer receipt still owns its exact R prelude until its true phase
	// completes. Do not permit a new invocation to overwrite that tuple.
	return nil
}
func (m *MultivariatePolynomialManager) Divides(q, p Polynomial) (bool, error) {
	if e := m.ready(); e != nil {
		return false, e
	}
	return m.core.Divides(q, p)
}
func (m *MultivariatePolynomialCore) Divides(q, p Polynomial) (bool, error) {
	return m.dividesWithReceipt(q, p, nil)
}
func (m *MultivariatePolynomialCore) dividesWithReceipt(q, p Polynomial, receipt *polynomialDividesInvocationReceipt) (answer bool, err error) {
	if e := m.check(); e != nil {
		return false, e
	}
	var entry polynomialDividesTraceInvocation
	if e := m.traceDividesEntry(q, p, &entry); e != nil {
		if receipt != nil && entry.core != nil {
			receipt.trace = entry
		}
		return false, e
	}
	if receipt != nil && entry.core != nil {
		receipt.trace = entry
	}
	zero, e := m.IsZero(p)
	if e != nil {
		return false, e
	}
	if zero {
		return true, nil
	}
	if e = polynomialDividesSourceAssert(func() (bool, error) {
		zero, e = m.IsZero(q)
		return !zero, e
	}); e != nil {
		return false, e
	}
	s := &m.dividesLocal
	if s.active || s.invocation == ^uint64(0) {
		return false, ErrPolynomialPending
	}
	invocation := s.invocation + 1
	*s = polynomialDividesScope{active: true, executing: true, invocation: invocation}
	s.directReceipt = polynomialDividesInvocationReceipt{core: m, frame: s, invocation: invocation, trace: entry}
	if receipt != nil {
		*receipt = s.directReceipt
	}
	defer func() {
		s.executing = false
		s.bodyCause = err
		err = errors.Join(err, m.finishDividesSource())
		if err == nil {
			err = m.som.transferArithmeticRowsToMember(&s.directReceipt.prelude)
			if err == nil && !m.dividesTraceInvocationPending(&s.directReceipt.trace) && s.stage == 2 && !s.monomial.assigning && !numericIntegerManager_dividesNumericPending(m.integer, &s.directReceipt) && s.generated == (Monomial{}) && s.calleeGeneration == 0 {
				s.active = false
			}
		}
		if receipt != nil {
			receipt.prelude = s.directReceipt.prelude
			receipt.trace = s.directReceipt.trace
		}
	}()
	if e = m.som.reset(); e != nil {
		return false, e
	}
	if e = m.som2.reset(); e != nil {
		return false, e
	}
	if e = m.som.acquireArithmeticInvocation(&s.directReceipt.prelude); e != nil {
		return false, e
	}
	if receipt != nil {
		receipt.prelude = s.directReceipt.prelude
	}
	if e = m.som.addPolynomialWithArithmeticReceipt(p, &s.directReceipt.prelude); e != nil {
		return false, e
	}
	maxQ, e := m.gradedLexMaxPosition(q)
	if e != nil {
		return false, e
	}
	mq, e := m.monomialCore(q, maxQ)
	if e != nil {
		return false, e
	}
	ql, e := m.polynomialLocation(q)
	if e != nil {
		return false, e
	}
	if e = m.cellIndexAssert(q, maxQ, 1511); e != nil {
		return false, e
	}
	aq := unumeral.IntegerArrayView{Allocator: m.monomials.allocator, Object: ql.raw, Count: 1, Offset: 12 + uint64(maxQ)*12, Stride: 12}
	// Exact source declaration order: monomial_ref, then scoped numeral.
	s.constructed = 1
	s.constructed = 2
	for {
		if e = m.checkpoint(); e != nil {
			return false, e
		}
		maxR, e := m.som.gradedLexMaxPosition()
		if e != nil {
			return false, e
		}
		if maxR == ^uint32(0) {
			return true, nil
		}
		mr, e := m.som.ms.monomial(m, maxR)
		if e != nil {
			return false, e
		}
		if polynomialSourceDebug {
			n, e := m.som.as.size(m)
			if e != nil {
				return false, e
			}
			if maxR >= n {
				return false, ErrPolynomialAssertion
			}
		}
		quotient, divides, e := m.dividesCreatedQuotient(mr, mq, &s.directReceipt)
		if e != nil {
			return false, e
		}
		if e = s.monomial.assign(m, quotient); e != nil {
			return false, e
		}
		if e = m.traceDividesMonomials(mr, mq, quotient, &s.directReceipt.trace); e != nil {
			return false, e
		}
		if !divides {
			return false, nil
		}
		divisor, e := aq.CheckedAt(m.memory, 0)
		if e != nil {
			return false, e
		}
		remainder, e := m.som.as.coefficient(m, maxR)
		if e != nil {
			return false, e
		}
		ok, e := numericIntegerModularManager_dividesWithDividesReceipt(m.modular, &divisor, &remainder, &s.directReceipt)
		if e != nil {
			return false, e
		}
		if !ok {
			return false, nil
		}
		remainder, e = m.som.as.coefficient(m, maxR)
		if e != nil {
			return false, e
		}
		divisor, e = aq.CheckedAt(m.memory, 0)
		if e != nil {
			return false, e
		}
		e = numericIntegerModularManager_divWithDividesReceipt(m.modular, &remainder, &divisor, &s.numeral, &s.directReceipt)
		if e != nil {
			return false, e
		}
		if e = numericIntegerModularManager_negWithDividesReceipt(m.modular, &s.numeral, &s.directReceipt); e != nil {
			return false, e
		}
		if e = (m.som).addMulWithArithmeticReceipt(&s.numeral, quotient, q, &s.directReceipt.prelude); e != nil {
			return false, e
		}
	}
}
