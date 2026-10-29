package polynomial

// Source: src/math/polynomial/polynomial.cpp:5094-5156; src/util/obj_ref.h.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

var ErrPolynomialUnreachable = errors.New("mpnum: polynomial source compiler unreachable")

type polynomialDivisionReference struct {
	value, incoming                              Monomial
	decrementStarted, incomingStarted, assigning bool
	cause                                        error
}

func (r *polynomialDivisionReference) drop(m *MultivariatePolynomialCore, p *Monomial, started *bool, owning bool) error {
	if *p == (Monomial{}) {
		return nil
	}
	if *started {
		if !owning {
			return r.cause
		}
		if e := m.monomials.CleanupValue(p); e != nil {
			return e
		}
		*started = false
		return nil
	}
	e := m.monomials.DecRef(*p)
	if e != nil {
		if l, x := m.monomials.location(*p); x == nil && l.stage != 0 {
			*started = true
		}
		r.cause = e
		return e
	}
	*p = Monomial{}
	return nil
}
func (r *polynomialDivisionReference) assign(m *MultivariatePolynomialCore, p Monomial) error {
	if r.assigning {
		return ErrPolynomialPending
	}
	if p != (Monomial{}) {
		if e := m.monomials.IncRef(p); e != nil {
			return e
		}
	}
	r.incoming = p
	r.assigning = true
	if m.generatedActive && m.generatedMonomial == p {
		m.transferGeneratedMonomial()
	}
	if e := r.drop(m, &r.value, &r.decrementStarted, false); e != nil {
		return e
	}
	r.value = r.incoming
	r.incoming = Monomial{}
	r.assigning = false
	return nil
}

// Both records are actual method locals embedded in the charged core header.
// Scalar exact_div's numeral is RAW: no source Del on pre-end exceptions.
type polynomialExactDivisionScope struct {
	traceGenerated           Monomial
	calleeGeneration         uint64
	invocation               uint64
	pscOwner                 uint64
	monomial                 polynomialDivisionReference
	numeral                  unumeral.Int
	numeric                  polynomialNumericRoots
	normalize                unumeral.IntegerNormalizeInvocationReceipt
	divisionRaw, extendedRaw unumeral.IntegerRawInvocationReceipt

	boolNumeral                          unumeral.IntegerDividesNumeralInvocationReceipt
	scalarReceipt                        polynomialScalarExactDivInvocationReceipt
	directReceipt                        polynomialExactDivInvocationReceipt
	output                               polynomialArithmeticInvocationReceipt
	executing                            bool
	returned                             Polynomial
	numericPhase                         uint8
	kind, stage                          uint8
	active, endEntered                   bool
	ownStage                             uint8
	ownStarted                           bool
	bodyCause, sourceCause, cleanupCause error
}

func (m *MultivariatePolynomialCore) generatedQuotient(a, b Monomial) (Monomial, bool, error) {
	return m.generatedQuotientForExactDiv(a, b, nil)
}
func (m *MultivariatePolynomialCore) generatedQuotientForExactDiv(a, b Monomial, receipt *polynomialExactDivInvocationReceipt) (Monomial, bool, error) {
	if m.generatedActive {
		return Monomial{}, false, ErrPolynomialPending
	}
	outcome := monomialFactoryOutcome{}
	r, ok, e := m.monomials.divideCreated(a, b, &outcome)
	if e != nil && outcome.started {
		m.recordGeneratedCallee(outcome)
		if m.matchesExactDivInvocation(receipt) && receipt.frame.executing {
			receipt.frame.calleeGeneration = outcome.generation
		}
	}
	if e == nil && outcome.created {
		m.generatedMonomial = r
		m.generatedActive = true
		m.generatedPSC = m.pscExecutingOwner()
	}
	return r, ok, e
}
func (m *MultivariatePolynomialCore) exactDivisionCleanup(owning bool) error {
	return m.exactDivisionCleanupPhase(owning, owning)
}
func (m *MultivariatePolynomialCore) exactDivisionCleanupPhase(owning, abort bool) error {
	s := &m.exactDivision
	if s.kind == 2 {
		if !owning {
			// Source tmp DEL is end-only and the scalar body executes it at
			// its literal frontier. A stopped source suffix stays unread.
			return s.sourceCause
		}
		return m.cleanupScalarExactDivInvocation(&s.scalarReceipt, abort)
	}
	if !s.active {
		return nil
	}
	if s.kind == 2 && !abort && !s.endEntered {
		return nil
	}
	if s.executing {
		return ErrPolynomialPending
	}
	if !owning && s.sourceCause != nil {
		return s.sourceCause
	}
	if s.kind == 1 {
		if owning {
			if e := m.cleanupArithmeticInvocation(&s.directReceipt.prelude, false); e != nil {
				return e
			}
			if e := m.cleanupArithmeticInvocation(&s.output, false); e != nil {
				return e
			}
			if e := numericIntegerManager_cleanupExactDivNumeric(m.integer, &s.directReceipt, false); e != nil {
				return e
			}
		} else if e := m.exactDivChildSourceCause(s); e != nil {
			s.sourceCause = e
			return e
		}
	}
	cursor := &s.stage
	if owning {
		if !s.ownStarted {
			s.ownStage = s.stage
			s.ownStarted = true
		}
		cursor = &s.ownStage
	}
	if *cursor == 0 {
		var e error
		if owning {
			e = m.deleteNumeral(&s.numeral)
		} else {
			_, e = upolynomialSourceNumeralDel(m.integer, &s.numeral)
		}
		if e != nil {
			if !owning {
				s.sourceCause = e
			}
			return e
		}
		*cursor = 1
	}
	if s.kind == 1 && *cursor == 1 {
		if e := s.monomial.drop(m, &s.monomial.value, &s.monomial.decrementStarted, owning); e != nil {
			if !owning {
				s.sourceCause = e
			}
			return e
		}
		*cursor = 2
	}
	pendingNumeric := s.traceGenerated != (Monomial{}) || s.numeric.PendingObject().OwnerID() != 0
	for _, raw := range s.numeric.PendingAllocatorBlocks() {
		pendingNumeric = pendingNumeric || raw.OwnerID() != 0
	}
	if s.kind == 1 {
		pendingNumeric = pendingNumeric || s.calleeGeneration != 0 || m.exactDivDirectPending(s) || m.matchesArithmeticInvocation(&s.output)
	}
	if !abort && (s.monomial.assigning || pendingNumeric) {
		return nil
	}
	if abort {
		if s.traceGenerated != (Monomial{}) {
			if m.generatedActive && m.generatedMonomial == s.traceGenerated {
				if e := m.cleanupGeneratedMonomial(); e != nil {
					return e
				}
			}
			s.traceGenerated = Monomial{}
		}
		if s.kind == 1 {
			if e := numericIntegerManager_cleanupExactDivNumeric(m.integer, &s.directReceipt, true); e != nil {
				return e
			}
			if e := m.cleanupArithmeticInvocation(&s.output, true); e != nil {
				return e
			}
		}
		if s.calleeGeneration != 0 {
			if m.monomials.creation.generation == s.calleeGeneration && m.monomials.creation.parent == m {
				if e := m.monomials.CleanupCreation(); e != nil {
					return e
				}
			}
			s.calleeGeneration = 0
		}
		if e := s.numeric.cleanup(m.integer); e != nil {
			return e
		}
		if e := s.monomial.drop(m, &s.monomial.incoming, &s.monomial.incomingStarted, true); e != nil {
			return e
		}
		s.monomial.assigning = false
		if s.returned != (Polynomial{}) {
			if e := m.CleanupValue(&s.returned); e != nil {
				return e
			}
		}
	}
	if s.returned != (Polynomial{}) && !abort {
		return nil
	}
	if owning {
		s.active = false
		// The matched cohort borrow ends with the actual owning continuation;
		// retain source cause/cursor independently for diagnostics.
		s.pscOwner = 0
		s.cleanupCause = nil
		return nil
	}
	if s.kind == 1 && s.bodyCause != nil && m.matchesArithmeticInvocation(&s.directReceipt.prelude) {
		// The actual NUM/reference suffix is complete. Retain only its
		// diagnostic history and embedded prelude receipt: R is an earlier
		// persistent member invocation, not a still-live lexical NUM.
		s.active = false
		s.pscOwner = 0
		return nil
	}
	*s = polynomialExactDivisionScope{invocation: s.invocation}
	return nil
}
func (m *MultivariatePolynomialCore) CleanupExactDivision() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.exactDivTrace.running || m.exactDivision.executing {
		return ErrPolynomialPending
	}
	if e := m.cleanupExactDivTrace(&m.exactDivTrace.current); e != nil {
		return e
	}
	e := m.cleanupExactDivInvocation(&m.exactDivision.directReceipt, true)
	if e == nil && m.exactDivision.kind == 2 {
		e = m.exactDivisionCleanup(true)
	}
	if e != nil {
		m.exactDivision.cleanupCause = e
	}
	return e
}
func (m *MultivariatePolynomialCore) ExactDiv(p, q Polynomial) (Polynomial, error) {
	return m.exactDivWithReceipt(p, q, nil)
}
func (m *MultivariatePolynomialCore) exactDivWithReceipt(p, q Polynomial, receipt *polynomialExactDivInvocationReceipt) (result Polynomial, err error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	var entry polynomialExactDivTraceInvocation
	if e := m.traceExactDivEntry(p, q, &entry); e != nil {
		if receipt != nil && entry.core != nil {
			receipt.trace = entry
		}
		return Polynomial{}, e
	}
	if receipt != nil && entry.core != nil {
		receipt.trace = entry
	}
	zero, e := m.IsZero(p)
	if e != nil {
		return Polynomial{}, e
	}
	if zero {
		return p, nil
	}
	if e = polynomialExactDivSourceAssert(5120, "!is_zero(q)", func() (bool, error) {
		zero, e = m.IsZero(q)
		return !zero, e
	}); e != nil {
		return Polynomial{}, e
	}
	if m.exactDivision.active {
		return Polynomial{}, ErrPolynomialPending
	}
	if receipt == nil {
		receipt = &m.exactDivision.directReceipt
	}
	if e = m.som.reset(); e != nil {
		return Polynomial{}, e
	}
	if e = m.som2.reset(); e != nil {
		return Polynomial{}, e
	}
	if receipt != nil {
		if e = m.som.acquireArithmeticInvocation(&receipt.prelude); e != nil {
			return Polynomial{}, e
		}
		// This cold member records only the actual prelude acquisition. No
		// source NUM/reference declaration has been constructed at this point.
		m.exactDivision.directReceipt.prelude = receipt.prelude
	}
	var prelude *polynomialArithmeticInvocationReceipt
	if receipt != nil {
		prelude = &receipt.prelude
	}
	if e = m.som.addPolynomialWithArithmeticReceipt(p, prelude); e != nil {
		return Polynomial{}, e
	}
	maxQ, e := m.gradedLexMaxPosition(q)
	if e != nil {
		return Polynomial{}, e
	}
	monomialQ, e := m.monomialCore(q, maxQ)
	if e != nil {
		return Polynomial{}, e
	}
	coefficientQ, e := m.coefficientCore(q, maxQ)
	if e != nil {
		return Polynomial{}, e
	}
	s := &m.exactDivision
	if s.invocation == ^uint64(0) {
		return Polynomial{}, ErrPolynomialPending
	}
	preludeIdentity := receipt.prelude
	*s = polynomialExactDivisionScope{active: true, executing: true, kind: 1, pscOwner: m.pscExecutingOwner(), invocation: s.invocation + 1}
	s.directReceipt = polynomialExactDivInvocationReceipt{core: m, frame: s, invocation: s.invocation, prelude: preludeIdentity, trace: entry}
	*receipt = s.directReceipt
	defer func() {
		s.executing = false
		s.bodyCause = err
		cleanup := m.exactDivisionCleanup(false)
		if cleanup == nil && err == nil {
			// R.reset has completed every source row action, and the actual
			// NUM/reference suffix has now completed. Acknowledge only this
			// reached prelude before allowing another invocation to reuse R.
			cleanup = m.cleanupArithmeticInvocation(&preludeIdentity, true)
		}
		if cleanup != nil {
			s.cleanupCause = cleanup
			// The source evaluates C.mk before destroying locals. Its actual new
			// returned object remains named if a later local destruction refuses.
			if result != (Polynomial{}) {
				s.returned = result
				result = Polynomial{}
			}
			err = errors.Join(err, cleanup)
		}
	}()
	for {
		if e = m.checkpoint(); e != nil {
			return Polynomial{}, e
		}
		maxR, e := m.som.gradedLexMaxPosition()
		if e != nil {
			return Polynomial{}, e
		}
		if maxR == ^uint32(0) {
			if e = m.som.reset(); e != nil {
				return Polynomial{}, e
			}
			return m.som2.mkWithArithmeticReceipt(false, &s.output)
		}
		monomialR, e := m.som.ms.monomial(m, maxR)
		if e != nil {
			return Polynomial{}, e
		}
		coefficientR, e := m.som.as.coefficient(m, maxR)
		if e != nil {
			return Polynomial{}, e
		}
		quotient, divides, e := m.generatedQuotientForExactDiv(monomialR, monomialQ, &s.directReceipt)
		if e != nil {
			return Polynomial{}, e
		}
		// VERIFY evaluates in every profile. The selected GNU release profile
		// invokes __builtin_unreachable on false; this is a typed UB guard.
		if !divides {
			return Polynomial{}, polynomialExactDivSourceVerifyFailure()
		}
		if e = m.traceExactDivMonomials(monomialR, monomialQ, quotient, &s.directReceipt.trace); e != nil {
			if m.generatedActive && m.generatedMonomial == quotient {
				s.traceGenerated = quotient
			}
			return Polynomial{}, e
		}
		if e = s.monomial.assign(m, quotient); e != nil {
			return Polynomial{}, e
		}
		s.numericPhase = 1
		if e = numericIntegerModularManager_divWithExactDivReceipt(m.modular, &coefficientR, &coefficientQ, &s.numeral, &s.directReceipt); e != nil {
			return Polynomial{}, e
		}
		s.numericPhase = 2
		if s.output.invocation == 0 {
			if e = m.som2.acquireArithmeticInvocation(&s.output); e != nil {
				return Polynomial{}, e
			}
		}
		if e = (m.som2).addWithArithmeticReceipt(&s.numeral, quotient, &s.output); e != nil {
			return Polynomial{}, e
		}
		s.numericPhase = 3
		if e = numericIntegerModularManager_negWithExactDivReceipt(m.modular, &s.numeral, &s.directReceipt); e != nil {
			return Polynomial{}, e
		}
		s.numericPhase = 4
		if e = (m.som).addMulWithArithmeticReceipt(&s.numeral, quotient, q, &s.directReceipt.prelude); e != nil {
			return Polynomial{}, e
		}
	}
}

func (m *MultivariatePolynomialCore) exactDivDirectPending(s *polynomialExactDivisionScope) bool {
	if m.modular.MatchesPNormalizeInvocation(&s.normalize) || m.integer.MatchesRawInvocation(&s.divisionRaw) || m.integer.MatchesRawInvocation(&s.extendedRaw) {
		return true
	}

	return false
}

// These are actual called children; no foreign/global active frame is read.
func (m *MultivariatePolynomialCore) exactDivChildSourceCause(s *polynomialExactDivisionScope) error {
	if e := m.arithmeticInvocationSourceCause(&s.directReceipt.prelude); e != nil {
		return e
	}
	if e := numericIntegerManager_exactDivNumericSourceCause(m.integer, &s.directReceipt); e != nil {
		return e
	}
	return m.arithmeticInvocationSourceCause(&s.output)
}
