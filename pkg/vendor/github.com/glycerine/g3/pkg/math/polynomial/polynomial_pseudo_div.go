package polynomial

// Source: src/math/polynomial/polynomial.cpp:4905-5091; src/util/obj_ref.h.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

const (
	pseudoOldP uint8 = iota
	pseudoOldQ
	pseudoQuotient
	pseudoLeading
	pseudoPowered
	pseudoLeft
	pseudoRight
	pseudoLeadB
	pseudoReductB
	pseudoAdjusted
	pseudoConstantPower
	pseudoNumeral
	pseudoMonomial
)

// These are actual possible source locals embedded in the charged core header.
// Each record is constructed only at its source site. The scalar stack records
// lexical destruction order. At most seven records in this source scope are
// simultaneously constructed: DEBUG wrapper3 + core NUM/lead/reduct + inner
// monomial, or the seven DEBUG verification references after core destruction.
// Nested Power has its own existing result record. The candidate slots are a
// charged Go continuation representation, not the C++ stack allocation geometry.
type polynomialPseudoDivisionScope struct {
	traceDisplay                          polynomialDisplayInvocationReceipt
	modD                                  polynomialArithmeticInvocationReceipt
	operation                             polynomialArithmeticInvocationReceipt
	generated                             Monomial
	calleeGeneration                      uint64
	monomialAllocator                     monomialAllocatorInvocationReceipt
	remainderBuffer                       polynomialArithmeticInvocationReceipt
	quotientBuffer                        polynomialArithmeticInvocationReceipt
	normalize                             unumeral.IntegerNormalizeInvocationReceipt
	directReceipt                         polynomialPseudoDivisionInvocationReceipt
	numeric                               polynomialNumericRoots
	numericPhase                          uint8
	invocation                            uint64
	executing                             bool
	sourceCause                           error
	pscOwner                              uint64
	power                                 polynomialPowerInvocationReceipt
	coefficient                           polynomialCoefficientInvocationReceipt
	coefficientAppend                     polynomialCheapAppendInvocationReceipt
	coefficientFactory                    polynomialFactoryInvocationReceipt
	refs                                  [11]polynomialReference
	numeral                               unumeral.Int
	monomial                              polynomialDivisionReference
	stack                                 [13]uint8
	depth                                 uint8
	constructed                           uint16
	active, numeralStopped                bool
	bodyCause, cleanupCause, numeralCause error
}

func (s *polynomialPseudoDivisionScope) construct(which uint8) {
	s.constructed |= uint16(1) << which
	s.stack[s.depth] = which
	s.depth++
}
func (m *MultivariatePolynomialCore) pseudoPop(owning bool) error {
	s := &m.pseudoDivision
	if s.depth == 0 {
		return nil
	}
	id := s.stack[s.depth-1]
	var e error
	switch id {
	case pseudoNumeral:
		if s.numeralStopped && !owning {
			return s.numeralCause
		}
		if owning {
			e = m.deleteNumeral(&s.numeral)
		} else {
			_, e = upolynomialSourceNumeralDel(m.integer, &s.numeral)
		}
		if e != nil {
			s.numeralStopped = true
			s.numeralCause = e
		}
	case pseudoMonomial:
		e = s.monomial.drop(m, &s.monomial.value, &s.monomial.decrementStarted, owning)
	default:
		e = s.refs[id].closeValue(m, owning)
	}
	if e != nil {
		if !owning && s.sourceCause == nil {
			s.sourceCause = e
		}
		return e
	}
	s.constructed &^= uint16(1) << id
	s.depth--
	return nil
}
func (m *MultivariatePolynomialCore) pseudoCleanup(owning bool) error {
	return m.pseudoCleanupPhase(owning, owning)
}
func (m *MultivariatePolynomialCore) pseudoCleanupPhase(owning, abort bool) error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	s := &m.pseudoDivision
	if !owning && s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	if s.executing {
		return ErrPolynomialPending
	}
	if !owning && s.sourceCause != nil {
		return s.sourceCause
	}
	if owning {
		if e := m.cleanupDisplayReceipt(&s.traceDisplay); e != nil {
			return e
		}
	} else if cause := m.displayReceiptSourceCause(&s.traceDisplay); cause != nil {
		s.sourceCause = cause
		return cause
	}
	for _, child := range []*polynomialArithmeticInvocationReceipt{&s.modD, &s.operation, &s.quotientBuffer, &s.remainderBuffer} {
		if !owning {
			if cause := m.arithmeticInvocationSourceCause(child); cause != nil {
				if s.sourceCause == nil {
					s.sourceCause = cause
				}
				return cause
			}
		}
		if e := m.cleanupArithmeticInvocation(child, false); e != nil {
			return e
		}
	}
	if e := m.modular.CleanupPNormalizeInvocation(&s.normalize, false); e != nil {
		return e
	}
	if e := m.cleanupCoefficientInvocation(&s.coefficient, false); e != nil {
		return e
	}
	if !owning {
		if cause := m.powerInvocationSourceCause(&s.power); cause != nil {
			return cause
		}
	} else if e := m.cleanupPowerInvocation(&s.power, false); e != nil {
		return e
	}
	for s.depth != 0 {
		if e := m.pseudoPop(owning); e != nil {
			return e
		}
	}
	// Unjoined INCs are absent from ordinary source destructors.
	if abort {
		for i := len(s.refs) - 1; i >= 0; i-- {
			if e := s.refs[i].abort(m); e != nil {
				return e
			}
		}
		if e := s.monomial.drop(m, &s.monomial.incoming, &s.monomial.incomingStarted, true); e != nil {
			return e
		}
		s.monomial.assigning = false
	}
	if abort {
		for _, child := range []*polynomialArithmeticInvocationReceipt{&s.modD, &s.operation, &s.quotientBuffer, &s.remainderBuffer} {
			if e := m.cleanupArithmeticInvocation(child, true); e != nil {
				return e
			}
		}
		if e := m.modular.CleanupPNormalizeInvocation(&s.normalize, true); e != nil {
			return e
		}
		if e := m.cleanupPseudoCreated(&s.directReceipt); e != nil {
			return e
		}
		if e := s.numeric.cleanup(m.integer); e != nil {
			return e
		}
		if e := m.cleanupPowerInvocation(&s.power, true); e != nil {
			return e
		}
		if e := m.cleanupCoefficientInvocation(&s.coefficient, true); e != nil {
			return e
		}
		if e := m.cleanupFactoryInvocation(&s.coefficientFactory, true); e != nil {
			return e
		}
		if e := m.cheap.cleanupAppendInvocation(&s.coefficientAppend, true); e != nil {
			return e
		}
	}
	for i := range s.refs {
		if s.refs[i].assigning || s.refs[i].incoming != (Polynomial{}) {
			return nil
		}
	}
	if s.monomial.assigning || s.monomial.incoming != (Monomial{}) {
		return nil
	}
	if r := &s.coefficient; r.manager == m && r.frame == &m.coefficientInvocation && r.invocation != 0 && r.invocation == r.frame.invocation && (r.frame.active || r.frame.bufferActive) {
		return nil
	}
	if m.matchesPowerInvocation(&s.power) {
		return nil
	}
	if m.matchesArithmeticInvocation(&s.modD) || m.matchesArithmeticInvocation(&s.operation) || m.matchesArithmeticInvocation(&s.quotientBuffer) || m.matchesArithmeticInvocation(&s.remainderBuffer) {
		return nil
	}
	if polynomialNegNumericPending(&s.numeric) || m.modular.MatchesPNormalizeInvocation(&s.normalize) {
		return nil
	}
	if m.displayReceiptPending(&s.traceDisplay) {
		return nil
	}
	*s = polynomialPseudoDivisionScope{traceDisplay: s.traceDisplay, invocation: s.invocation, sourceCause: s.sourceCause, bodyCause: s.bodyCause, cleanupCause: s.cleanupCause, numeralCause: s.numeralCause, directReceipt: s.directReceipt, monomialAllocator: s.monomialAllocator, coefficientAppend: s.coefficientAppend, coefficientFactory: s.coefficientFactory, power: s.power, coefficient: s.coefficient, normalize: s.normalize, remainderBuffer: s.remainderBuffer, quotientBuffer: s.quotientBuffer, operation: s.operation, modD: s.modD}
	return nil
}
func (m *MultivariatePolynomialCore) CleanupPseudoDivision() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.pseudoDivision.executing {
		return ErrPolynomialPending
	}
	e := m.pseudoCleanup(true)
	if e != nil {
		m.pseudoDivision.cleanupCause = e
	}
	return e
}
func (m *MultivariatePolynomialCore) generatedDivideVariablePower(a Monomial, x, k uint32) (Monomial, error) {
	return m.generatedDivideVariablePowerWithReceipt(a, x, k, nil)
}
func (m *MultivariatePolynomialCore) generatedDivideVariablePowerWithReceipt(a Monomial, x, k uint32, r *polynomialPseudoDivisionInvocationReceipt) (Monomial, error) {
	if r != nil && (!m.matchesPseudoInvocation(r) || !r.frame.executing || m.callbackDepth != 0) {
		return Monomial{}, ErrPolynomialOwner
	}
	if m.generatedActive {
		return Monomial{}, ErrPolynomialPending
	}
	outcome := monomialFactoryOutcome{}
	p, e := m.monomials.variableTransformCreated(a, x, k, 0, &outcome)
	if r != nil && outcome.allocatorInvocation.invocation != 0 {
		r.frame.monomialAllocator = outcome.allocatorInvocation
	}
	if e != nil && outcome.started {
		m.recordGeneratedCallee(outcome)
		if r != nil {
			r.frame.calleeGeneration = outcome.generation
		}
	}
	if e == nil && outcome.created {
		m.generatedMonomial = p
		m.generatedActive = true
		m.generatedPSC = m.pscExecutingOwner()
		if r != nil {
			r.frame.generated = p
			l, readErr := m.monomials.location(p)
			if readErr != nil {
				return p, readErr
			}
			l.pseudoCohort = r.frame
			l.pseudoInvocation = r.invocation
		}
	}
	return p, e
}
func (m *MultivariatePolynomialCore) pseudoEntry(q, r *PolynomialRef) error {
	return m.pseudoEntryWithReceipt(q, r, nil)
}
func (m *MultivariatePolynomialCore) pseudoEntryWithReceipt(q, r *PolynomialRef, receipt *polynomialPseudoDivisionInvocationReceipt) error {
	if e := m.check(); e != nil {
		return e
	}
	if r == nil || r.owner != m {
		return ErrPolynomialOwner
	}
	if e := r.open(); e != nil {
		return e
	}
	if q != nil {
		if q.owner != m {
			return ErrPolynomialOwner
		}
		if e := q.open(); e != nil {
			return e
		}
	}
	if m.pseudoDivision.active || m.powerLocal.active || m.pseudoDivision.invocation == ^uint64(0) {
		return ErrPolynomialPending
	}
	next := m.pseudoDivision.invocation + 1
	m.pseudoDivision = polynomialPseudoDivisionScope{active: true, executing: true, invocation: next, pscOwner: m.pscExecutingOwner()}
	m.pseudoDivision.directReceipt = polynomialPseudoDivisionInvocationReceipt{m, &m.pseudoDivision, next}
	if receipt != nil {
		*receipt = m.pseudoDivision.directReceipt
	}
	return nil
}
func (m *MultivariatePolynomialCore) pseudoFinish(err *error) {
	s := &m.pseudoDivision
	s.executing = false
	s.bodyCause = *err
	if s.cleanupCause != nil {
		*err = errors.Join(*err, s.cleanupCause)
		return
	}
	e := m.pseudoCleanup(false)
	if e != nil {
		s.cleanupCause = e
		if s.sourceCause == nil {
			s.sourceCause = e
		}
		*err = errors.Join(*err, e)
	}
}
func (m *MultivariatePolynomialCore) PseudoDivide(p, q Polynomial, x uint32, d *uint32, Q, R *PolynomialRef) (err error) {
	if d == nil || Q == nil {
		return ErrPolynomialOwner
	}
	if e := m.pseudoEntry(Q, R); e != nil {
		return e
	}
	defer m.pseudoFinish(&err)
	return m.pseudoCore(p, q, x, d, &Q.root, &R.root, false, true, false, nil)
}
func (m *MultivariatePolynomialCore) ExactPseudoDivide(p, q Polynomial, x uint32, Q, R *PolynomialRef) (err error) {
	if Q == nil {
		return ErrPolynomialOwner
	}
	if e := m.pseudoEntry(Q, R); e != nil {
		return e
	}
	defer m.pseudoFinish(&err)
	var d uint32
	return m.pseudoCore(p, q, x, &d, &Q.root, &R.root, true, true, false, nil)
}
func (m *MultivariatePolynomialCore) ExactPseudoRemainder(p, q Polynomial, x uint32, R *PolynomialRef) error {
	return m.exactPseudoRemainderWithReceipt(p, q, x, R, nil)
}
func (m *MultivariatePolynomialCore) exactPseudoRemainderWithReceipt(p, q Polynomial, x uint32, R *PolynomialRef, receipt *polynomialPseudoDivisionInvocationReceipt) (err error) {
	if e := m.pseudoEntryWithReceipt(nil, R, receipt); e != nil {
		return e
	}
	defer m.pseudoFinish(&err)
	s := &m.pseudoDivision
	s.construct(pseudoQuotient)
	var d uint32
	return m.pseudoCore(p, q, x, &d, &s.refs[pseudoQuotient], &R.root, true, false, false, nil)
}
func (m *MultivariatePolynomialCore) PseudoRemainder(p, q Polynomial, x uint32, d *uint32, R *PolynomialRef) error {
	return m.pseudoRemainderWithReceipt(p, q, x, d, R, nil)
}
func (m *MultivariatePolynomialCore) pseudoRemainderWithReceipt(p, q Polynomial, x uint32, d *uint32, R *PolynomialRef, receipt *polynomialPseudoDivisionInvocationReceipt) (err error) {
	if d == nil {
		return ErrPolynomialOwner
	}
	if e := m.pseudoEntryWithReceipt(nil, R, receipt); e != nil {
		return e
	}
	defer m.pseudoFinish(&err)
	s := &m.pseudoDivision
	if polynomialSourceDebug {
		s.construct(pseudoOldP)
		s.construct(pseudoOldQ)
		if e := s.refs[pseudoOldP].assign(m, p); e != nil {
			return e
		}
		if e := s.refs[pseudoOldQ].assign(m, q); e != nil {
			return e
		}
	}
	s.construct(pseudoQuotient)
	if e := m.pseudoCore(p, q, x, d, &s.refs[pseudoQuotient], &R.root, false, polynomialSourceDebug, false, nil); e != nil {
		return e
	}
	if !polynomialSourceDebug {
		return nil
	}
	s.construct(pseudoLeading)
	degree, e := m.Degree(q, x)
	if e != nil {
		return e
	}
	l, e := m.coeffCoreWithReceipts(s.refs[pseudoOldQ].value, x, degree, nil, &s.coefficient, &s.coefficientAppend, &s.coefficientFactory)
	if e != nil {
		return e
	}
	if e = s.refs[pseudoLeading].assignWithFactoryReceipt(m, l, &s.coefficientFactory); e != nil {
		return e
	}
	s.construct(pseudoPowered)
	if e = m.powerIntoWithReceipt(s.refs[pseudoLeading].value, *d, &s.refs[pseudoPowered], &s.power); e != nil {
		return e
	}
	s.construct(pseudoLeft)
	lhs, e := m.mulWithReceipt(s.refs[pseudoPowered].value, s.refs[pseudoOldP].value, &s.operation)
	if e != nil {
		return e
	}
	if e = s.refs[pseudoLeft].assign(m, lhs); e != nil {
		return e
	}
	s.construct(pseudoRight)
	rhs, e := m.mulWithReceipt(s.refs[pseudoQuotient].value, s.refs[pseudoOldQ].value, &s.operation)
	if e != nil {
		return e
	}
	if e = s.refs[pseudoRight].assign(m, rhs); e != nil {
		return e
	}
	rhs, e = m.addWithReceipt(s.refs[pseudoRight].value, R.root.value, &s.operation)
	if e != nil {
		return e
	}
	if e = s.refs[pseudoRight].assign(m, rhs); e != nil {
		return e
	}
	isEqual, e := m.Equal(s.refs[pseudoLeft].value, s.refs[pseudoRight].value)
	if e != nil {
		return e
	}
	if e = m.tracePseudoRemainderVerification(&R.root); e != nil {
		return e
	}
	return polynomialPseudoSourceAssert(5078, "is_eq", func() (bool, error) { return isEqual, nil })
}
func (m *MultivariatePolynomialCore) pseudoCore(p, q Polynomial, x uint32, d *uint32, Q, R *polynomialReference, exact, quotient, modulo bool, bounds *PolynomialVariableDegrees) (err error) {
	s := &m.pseudoDivision
	baseDepth := s.depth
	defer func() {
		if s.cleanupCause != nil {
			return
		}
		if cause := m.displayReceiptSourceCause(&s.traceDisplay); cause != nil {
			s.sourceCause, s.cleanupCause = cause, cause
			err = errors.Join(err, cause)
			return
		}
		for _, child := range []*polynomialArithmeticInvocationReceipt{&s.modD, &s.operation, &s.quotientBuffer, &s.remainderBuffer} {
			if cause := m.arithmeticInvocationSourceCause(child); cause != nil {
				s.cleanupCause = cause
				if s.sourceCause == nil {
					s.sourceCause = cause
				}
				err = errors.Join(err, cause)
				return
			}
			if e := m.cleanupArithmeticInvocation(child, false); e != nil {
				s.cleanupCause = e
				err = errors.Join(err, e)
				return
			}
		}

		if e := m.modular.CleanupPNormalizeInvocation(&s.normalize, false); e != nil {
			s.cleanupCause = e
			err = errors.Join(err, e)
			return
		}
		if cause := m.coefficientInvocationSourceCause(&s.coefficient); cause != nil {
			s.cleanupCause = cause
			if s.sourceCause == nil {
				s.sourceCause = cause
			}
			err = errors.Join(err, cause)
			return
		}
		if e := m.cleanupCoefficientInvocation(&s.coefficient, false); e != nil {
			s.cleanupCause = e
			err = errors.Join(err, e)
			return
		}
		if cause := m.powerInvocationSourceCause(&s.power); cause != nil {
			s.cleanupCause = cause
			if s.sourceCause == nil {
				s.sourceCause = cause
			}
			err = errors.Join(err, cause)
			return
		}
		for s.depth > baseDepth {
			if e := m.pseudoPop(false); e != nil {
				s.cleanupCause = e
				err = errors.Join(err, e)
				return
			}
		}
	}()
	if e := polynomialPseudoSourceAssert(4907, "is_valid(x)", func() (bool, error) {
		return m.monomials.IsValidVar(x)
	}); e != nil {
		return e
	}
	if e := polynomialPseudoSourceAssert(4908, "!ModD || x2d != 0", func() (bool, error) {
		return !modulo || bounds != nil, nil
	}); e != nil {
		return e
	}
	if e := m.tracePseudoEntry(p, q, x); e != nil {
		return e
	}
	degA, e := m.Degree(p, x)
	if e != nil {
		return e
	}
	degB, e := m.Degree(q, x)
	if e != nil {
		return e
	}
	if degB == 0 {
		if e = R.assign(m, m.zero); e != nil {
			return e
		}
		if quotient {
			if exact {
				*d = degA + 1
				if *d == 1 {
					return Q.assign(m, p)
				}
				s.construct(pseudoConstantPower)
				if e = m.powerIntoWithReceipt(q, *d-1, &s.refs[pseudoConstantPower], &s.power); e != nil {
					return e
				}
				product, e := m.mulWithReceipt(p, s.refs[pseudoConstantPower].value, &s.operation)
				if e != nil {
					return e
				}
				if e = Q.assign(m, product); e != nil {
					return e
				}
				if modulo {
					product, e = m.modDWithReceipt(Q.value, bounds, &s.modD)
					if e != nil {
						return e
					}
					return Q.assign(m, product)
				}
				return nil
			}
			*d = 1
			return Q.assign(m, p)
		}
		return nil
	}
	if degB > degA {
		if e = Q.assign(m, m.zero); e != nil {
			return e
		}
		if e = R.assign(m, p); e != nil {
			return e
		}
		*d = 0
	}
	s.construct(pseudoNumeral)
	s.construct(pseudoLeadB)
	s.construct(pseudoReductB)
	lead, e := m.coeffCoreWithReceipts(q, x, degB, &s.refs[pseudoReductB], &s.coefficient, &s.coefficientAppend, &s.coefficientFactory)
	if e != nil {
		return e
	}
	if e = s.refs[pseudoLeadB].assignWithFactoryReceipt(m, lead, &s.coefficientFactory); e != nil {
		return e
	}
	*d = 0
	if e = R.assign(m, p); e != nil {
		return e
	}
	if e = Q.assign(m, m.zero); e != nil {
		return e
	}
	for {
		if e = m.checkpoint(); e != nil {
			return e
		}
		if e := m.tracePseudoIteration(p, q, Q, R, d); e != nil {
			return e
		}
		degR, e := m.Degree(R.value, x)
		if e != nil {
			return e
		}
		if degB > degR {
			if exact {
				exactD := degA - degB + 1
				if e = polynomialPseudoSourceAssert(4968, "d <= exact_d", func() (bool, error) {
					return *d <= exactD, nil
				}); e != nil {
					return e
				}
				if *d < exactD {
					s.construct(pseudoAdjusted)
					if e = m.powerIntoWithReceipt(s.refs[pseudoLeadB].value, exactD-*d, &s.refs[pseudoAdjusted], &s.power); e != nil {
						return e
					}
					if e = m.tracePseudoAdjusted(); e != nil {
						return e
					}
					if quotient {
						product, e := m.mulWithReceipt(s.refs[pseudoAdjusted].value, Q.value, &s.operation)
						if e != nil {
							return e
						}
						if e = Q.assign(m, product); e != nil {
							return e
						}
						if modulo {
							product, e = m.modDWithReceipt(Q.value, bounds, &s.modD)
							if e != nil {
								return e
							}
							if e = Q.assign(m, product); e != nil {
								return e
							}
						}
					}
					product, e := m.mulWithReceipt(s.refs[pseudoAdjusted].value, R.value, &s.operation)
					if e != nil {
						return e
					}
					if e = R.assign(m, product); e != nil {
						return e
					}
					if modulo {
						product, e = m.modDWithReceipt(R.value, bounds, &s.modD)
						if e != nil {
							return e
						}
						if e = R.assign(m, product); e != nil {
							return e
						}
					}
				}
			}
			return nil
		}
		if e = m.som.reset(); e != nil {
			return e
		}
		if e = m.som.acquireArithmeticInvocation(&s.remainderBuffer); e != nil {
			return e
		}
		if e = m.som2.reset(); e != nil {
			return e
		}
		if quotient {
			if e = m.som2.acquireArithmeticInvocation(&s.quotientBuffer); e != nil {
				return e
			}
		}
		size, e := m.readWord(R.value, 8)
		if e != nil {
			return e
		}
		for i := uint32(0); i < size; i++ {
			term, e := m.monomialCore(R.value, i)
			if e != nil {
				return e
			}
			a, e := m.mutableCoefficientCore(R.value, i)
			if e != nil {
				return e
			}
			degree, e := m.monomials.DegreeOf(term, x)
			if e != nil {
				return e
			}
			if degree == degR {
				s.construct(pseudoMonomial)
				prime, e := m.generatedDivideVariablePowerWithReceipt(term, x, degB, &s.directReceipt)
				if e != nil {
					return e
				}
				if e = s.monomial.assign(m, prime); e != nil {
					return e
				}
				if e = m.tracePseudoReducedMonomial(term, x, degR, degB); e != nil {
					return e
				}
				if e = polynomialPseudoSourceAssert(5014, "m->degree_of(x) == deg_R", func() (bool, error) {
					before, e := m.monomials.DegreeOf(term, x)
					return before == degR, e
				}); e != nil {
					return e
				}
				if e = polynomialPseudoSourceAssert(5015, "m_prime->degree_of(x) == deg_R - deg_B", func() (bool, error) {
					after, e := m.monomials.DegreeOf(s.monomial.value, x)
					return after == degR-degB, e
				}); e != nil {
					return e
				}
				if quotient {
					if e = (m.som2).addWithArithmeticReceipt(&a, s.monomial.value, &s.quotientBuffer); e != nil {
						return e
					}
				}
				s.numericPhase = 1
				if e = numericIntegerModularManager_copyWithPseudoReceipt(m.modular, &s.numeral, &a, &s.directReceipt); e != nil {
					return e
				}
				s.numericPhase = 2
				if e = numericIntegerModularManager_negWithPseudoReceipt(m.modular, &s.numeral, &s.directReceipt); e != nil {
					return e
				}
				if e = (m.som).addMulWithArithmeticReceipt(&s.numeral, s.monomial.value, s.refs[pseudoReductB].value, &s.remainderBuffer); e != nil {
					return e
				}
				if e = m.pseudoPop(false); e != nil {
					s.cleanupCause = e
					return e
				}
			} else {
				if e = (m.som).addMulWithArithmeticReceipt(&a, term, s.refs[pseudoLeadB].value, &s.remainderBuffer); e != nil {
					return e
				}
			}
		}
		if modulo {
			if e = m.som.modD(bounds); e != nil {
				return e
			}
		}
		remainder, e := m.som.mkWithArithmeticReceipt(false, &s.remainderBuffer)
		if e != nil {
			return e
		}
		if e = R.assign(m, remainder); e != nil {
			return e
		}
		if quotient {
			size, e := m.readWord(Q.value, 8)
			if e != nil {
				return e
			}
			for i := uint32(0); i < size; i++ {
				a, e := m.mutableCoefficientCore(Q.value, i)
				if e != nil {
					return e
				}
				term, e := m.monomialCore(Q.value, i)
				if e != nil {
					return e
				}
				if e = (m.som2).addMulWithArithmeticReceipt(&a, term, s.refs[pseudoLeadB].value, &s.quotientBuffer); e != nil {
					return e
				}
			}
			if modulo {
				if e = m.som2.modD(bounds); e != nil {
					return e
				}
			}
			result, e := m.som2.mkWithArithmeticReceipt(false, &s.quotientBuffer)
			if e != nil {
				return e
			}
			if e = Q.assign(m, result); e != nil {
				return e
			}
		}
		*d++
	}
}
