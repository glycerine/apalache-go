package polynomial

// Source: src/math/polynomial/polynomial.cpp:1495-1509,5954-6068,7284-7286;
// src/util/scoped_numeral.h; src/util/obj_ref.h.

import (
	"errors"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Four actual scoped numerals, one RAW numeral, and the actual monomial_ref.
// The created cohort is charged Go abort metadata for returned zero-RC locals;
// it is not a source buffer or a source destructor repairing abandoned locals.
type polynomialSquareRootScope struct {
	factory                              polynomialFactoryInvocationReceipt
	monomialAllocator                    monomialAllocatorInvocationReceipt
	receipt                              polynomialSqrtInvocationReceipt
	numeric                              polynomialNumericRoots
	divisionRaw                          unumeral.IntegerRawInvocationReceipt
	invocation                           uint64
	orphanInvocations                    []uint64
	values                               [4]unumeral.Int
	two                                  unumeral.Int
	mask                                 uint8
	reference                            polynomialSquareRootReference
	referenceCompleted                   bool
	active, executing, stopped           bool
	stage                                uint8
	orphans                              []Monomial
	drain                                uint32
	perfectSquare                        unumeral.IntegerPerfectSquareInvocationReceipt
	calleeGeneration                     uint64
	bodyCause, sourceCause, cleanupCause error
}

func (s *polynomialSquareRootScope) constructNumeral(i uint8) {
	s.values[i] = unumeral.Int{}
	s.mask |= 1 << i
}

type polynomialSquareRootReference struct {
	value, incoming                       Monomial
	consumed, incomingConsumed, assigning bool
	cause                                 error
}

func (r *polynomialSquareRootReference) drop(c *MultivariatePolynomialCore, p *Monomial, consumed *bool, owning bool) error {
	if *p == (Monomial{}) {
		return nil
	}
	if *consumed {
		if !owning {
			return r.cause
		}
		if e := c.monomials.CleanupValue(p); e != nil {
			return e
		}
		*consumed = false
		return nil
	}
	out := monomialDecrementOutcome{}
	e := c.monomials.decRefCoreOutcome(*p, &out)
	if out.written {
		if out.last && e != nil {
			*consumed = true
		} else {
			*p = Monomial{}
		}
	}
	if e != nil {
		r.cause = e
		return e
	}
	*p = Monomial{}
	return nil
}
func (r *polynomialSquareRootReference) assign(c *MultivariatePolynomialCore, p Monomial) error {
	if r.assigning {
		return ErrPolynomialPending
	}
	if e := c.monomials.IncRef(p); e != nil {
		return e
	}
	r.incoming = p
	r.assigning = true
	if e := r.drop(c, &r.value, &r.consumed, false); e != nil {
		return e
	}
	r.value = r.incoming
	r.incoming = Monomial{}
	r.assigning = false
	return nil
}
func (m *MultivariatePolynomialCore) gradedLexMinPosition(p Polynomial) (uint32, error) {
	n, e := m.readWord(p, 8)
	if e != nil {
		return 0, e
	}
	if n == 0 {
		return ^uint32(0), nil
	}
	min := uint32(0)
	for i := uint32(1); i < n; i++ {
		a, e := m.monomialCore(p, i)
		if e != nil {
			return 0, e
		}
		b, e := m.monomialCore(p, min)
		if e != nil {
			return 0, e
		}
		cmp, e := m.monomials.GradedLex(a, b)
		if e != nil {
			return 0, e
		}
		if cmp < 0 {
			min = i
		}
	}
	return min, nil
}
func (m *MultivariatePolynomialManager) squareRootMonomial(op uint8, a, b Monomial) (Monomial, bool, error) {
	s := &m.squareRoot
	c := m.core
	var e error
	outcome := monomialFactoryOutcome{}
	var p Monomial
	ok := true
	switch op {
	case 0:
		p, ok, e = c.monomials.sqrtCreated(a, &outcome)
	case 1:
		p, ok, e = c.monomials.divideCreated(a, b, &outcome)
	case 2:
		p, e = c.monomials.mulCreated(a, b, &outcome)
	}
	if outcome.started {
		s.monomialAllocator = outcome.allocatorInvocation
	}
	if e != nil && outcome.started {
		c.recordGeneratedCallee(outcome)
		s.calleeGeneration = outcome.generation
	}
	if e == nil && outcome.created {
		idx := uint32(len(s.orphans))
		old := s.orphans
		s.orphans = append(s.orphans, p)
		if len(old) != 0 && &old[0] != &s.orphans[0] {
			clear(old)
		}
		s.orphanInvocations = append(s.orphanInvocations, s.invocation)
		l := &c.monomials.locations[p.slot-1]
		l.squareRootCohort = s
		l.squareRootSlot = idx
	}
	return p, ok, e
}

// Receipt capture surrounds only the actual VM call, not retained global state.
func (m *MultivariatePolynomialManager) squareRootNumeral(op uint8, a, b, out *unumeral.Int) (bool, error) {
	c := m.core
	z := c.integer
	s := &m.squareRoot
	context := integerAllocationInvocationContext{squareRoot: &s.receipt, squareRootInvocation: s.invocation}
	if context.roots(z) == nil {
		return false, ErrPolynomialOwner
	}
	arithmetic := c.modular.WithAllocationContext(context.utility(z))
	switch op {
	case 0:
		return arithmetic.IsPerfectSquare(a, out)
	case 1:
		return true, arithmetic.SetInt32(out, 2)
	case 2:
		return true, arithmetic.Mul(a, b, out)
	case 3:
		return true, arithmetic.Neg(out)
	case 4:
		return true, arithmetic.Div(a, b, out)
	}
	return false, ErrPolynomialAssertion
}

// The actual persistent buffer keeps its own coefficient fields. Only a newly
// started subordinate physical allocation is an invocation's abort receipt.
func (m *MultivariatePolynomialManager) squareRootAdd(b *polynomialSOM, a *unumeral.Int, t Monomial) error {
	if m == nil || m.self != m || m.core == nil || (b != &m.core.som && b != &m.core.som2) || b.owner != m.core {
		return ErrPolynomialOwner
	}
	s := &m.squareRoot
	context := integerAllocationInvocationContext{squareRoot: &s.receipt, squareRootInvocation: s.invocation}
	if context.roots(m.core.integer) == nil {
		return ErrPolynomialOwner
	}
	// Bind the actual caller before the reached arithmetic leaf. No shared
	// pending-allocation comparison grants ownership to this invocation.
	_ = context.utility(m.core.integer)
	return b.addConsumedReceipts(a, t, false, nil, &s.numeric, nil)
}
func (m *MultivariatePolynomialManager) squareRootTerm(p Polynomial, i uint32) (bool, error) {
	t, e := m.core.monomialCore(p, i)
	if e != nil {
		return false, e
	}
	ok, e := m.core.monomials.IsSquare(t)
	if e != nil || !ok {
		return ok, e
	}
	a, e := m.core.coefficientCore(p, i)
	if e != nil {
		return false, e
	}
	return m.squareRootNumeral(0, &a, nil, &m.squareRoot.values[0])
}
func (m *MultivariatePolynomialManager) finishSquareRoot(cause error) error {
	s := &m.squareRoot
	s.executing = false
	if s.bodyCause == nil {
		s.bodyCause = cause
	}
	z := m.core.integer
	if _, _, stopped := s.perfectSquare.State(z); stopped {
		s.stopped = true
		if s.sourceCause == nil {
			s.sourceCause = s.perfectSquare.SourceCause()
		}
	}
	if e := z.RawInvocationSourceCause(&s.divisionRaw); e != nil {
		s.stopped = true
		if s.sourceCause == nil {
			s.sourceCause = e
		}
	}
	if s.stopped {
		return errors.Join(cause, s.sourceCause)
	}
	if s.referenceCompleted {
		if e := s.reference.drop(m.core, &s.reference.value, &s.reference.consumed, false); e != nil {
			s.stopped = true
			s.sourceCause = e
			return errors.Join(cause, e)
		}
		s.referenceCompleted = false
	}
	for s.stage < 4 {
		i := uint8(3) - s.stage
		if s.mask&(1<<i) != 0 {
			if _, e := upolynomialSourceNumeralDel(z, &s.values[i]); e != nil {
				s.stopped = true
				s.sourceCause = e
				return errors.Join(cause, e)
			}
			s.mask &^= 1 << i
		}
		s.stage++
	}
	if cause == nil && !m.core.matchesFactoryInvocation(&s.factory) && s.calleeGeneration == 0 && !(m.core.monomials.matchesMonomialAllocatorInvocation(&s.monomialAllocator) && s.monomialAllocator.outcome.Pending != (umemory.Block{})) && !polynomialNegNumericPending(&s.numeric) && !z.MatchesRawInvocation(&s.divisionRaw) {
		s.active = false
	}
	return cause
}
func (m *MultivariatePolynomialManager) beginSquareRoot() error {
	return m.beginSquareRootWithReceipt(nil)
}
func (m *MultivariatePolynomialManager) beginSquareRootWithReceipt(receipt *polynomialSqrtInvocationReceipt) error {
	s := &m.squareRoot
	if s.active {
		return ErrPolynomialPending
	}
	if s.invocation == ^uint64(0) {
		return ErrPolynomialPending
	}
	nonce := s.invocation + 1
	orphans, tags, drain := s.orphans, s.orphanInvocations, s.drain
	live := false
	for _, a := range orphans {
		if a != (Monomial{}) {
			live = true
			break
		}
	}
	if !live {
		orphans = orphans[:0]
		tags = tags[:0]
		drain = 0
	}
	*s = polynomialSquareRootScope{invocation: nonce, active: true, executing: true, orphans: orphans, orphanInvocations: tags, drain: drain, mask: 1}
	m.core.squareRootOwner = s
	s.receipt = polynomialSqrtInvocationReceipt{manager: m, frame: s, invocation: s.invocation}
	if receipt != nil {
		*receipt = polynomialSqrtInvocationReceipt{manager: m, frame: s, invocation: s.invocation}
	}
	return nil
}
func (m *MultivariatePolynomialManager) Sqrt(p Polynomial, r *PolynomialRef) (bool, error) {
	return m.sqrtWithReceipt(p, r, nil)
}
func (m *MultivariatePolynomialManager) sqrtWithReceipt(p Polynomial, r *PolynomialRef, receipt *polynomialSqrtInvocationReceipt) (ok bool, cause error) {
	if e := m.ready(); e != nil {
		return false, e
	}
	if r == nil || r.self != r || r.owner != m.core || r.closed || r.closing {
		return false, ErrPolynomialOwner
	}
	if e := polynomialSqrtSourceAssert(5965, "p != 0", func() (bool, error) {
		return p != (Polynomial{}), nil
	}); e != nil {
		return false, e
	}
	c := m.core
	n, e := c.readWord(p, 8)
	if e != nil {
		return false, e
	}
	if n == 0 {
		return true, r.Assign(p)
	}
	s := &m.squareRoot
	if s.active {
		return false, ErrPolynomialPending
	}
	if e = m.beginSquareRootWithReceipt(receipt); e != nil {
		return false, e
	}
	defer func() { cause = m.finishSquareRoot(cause) }()
	if e := m.traceSqrtEntry(p); e != nil {
		return false, e
	}
	min, e := c.gradedLexMinPosition(p)
	if e != nil {
		return false, e
	}
	ok, e = m.squareRootTerm(p, min)
	if e != nil || !ok {
		return false, e
	}
	max, e := c.gradedLexMaxPosition(p)
	if e != nil {
		return false, e
	}
	ok, e = m.squareRootTerm(p, max)
	if e != nil || !ok {
		return false, e
	}
	term, e := c.monomialCore(p, max)
	if e != nil {
		return false, e
	}
	m1, ok, e := m.squareRootMonomial(0, term, Monomial{})
	if e != nil {
		return false, e
	}
	if e = polynomialSqrtSourceAssert(6006, "m1 != 0", func() (bool, error) {
		return ok, nil
	}); e != nil {
		return false, e
	}
	R, C := &c.som, &c.som2
	if e = R.reset(); e != nil {
		return false, e
	}
	if e = C.reset(); e != nil {
		return false, e
	}
	if _, e = m.squareRootNumeral(1, nil, nil, &s.two); e != nil {
		return false, e
	}
	s.constructNumeral(1)
	if _, e = m.squareRootNumeral(2, &s.values[0], &s.two, &s.values[1]); e != nil {
		return false, e
	}
	if e = m.squareRootAdd(R, &s.values[0], m1); e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		if i == max {
			continue
		}
		a, e := c.coefficientCore(p, i)
		if e != nil {
			return false, e
		}
		t, e := c.monomialCore(p, i)
		if e != nil {
			return false, e
		}
		if e = m.squareRootAdd(C, &a, t); e != nil {
			return false, e
		}
	}
	s.constructNumeral(2)
	s.constructNumeral(3)
	s.referenceCompleted = true
	for {
		s.executing = false
		e = c.checkpoint()
		s.executing = true
		if e != nil {
			return false, e
		}
		if e := m.traceSqrtBuffers(); e != nil {
			return false, e
		}
		pos, e := C.gradedLexMaxPosition()
		if e != nil {
			return false, e
		}
		if pos == ^uint32(0) {
			if e = C.reset(); e != nil {
				return false, e
			}
			out, e := R.mkWithFactoryReceipt(false, &s.factory)
			if e != nil {
				return false, e
			}
			if e = r.open(); e != nil {
				return false, e
			}
			if e = r.root.assignWithFactoryReceipt(c, out, &s.factory); e != nil {
				return false, e
			}
			return true, nil
		}
		t, e := C.ms.monomial(c, pos)
		if e != nil {
			return false, e
		}
		mi, divides, e := m.squareRootMonomial(1, t, m1)
		if e != nil {
			return false, e
		}
		if !divides {
			if e = R.reset(); e != nil {
				return false, e
			}
			if e = C.reset(); e != nil {
				return false, e
			}
			return false, nil
		}
		coefficient, e := C.as.coefficient(c, pos)
		if e != nil {
			return false, e
		}
		divides, e = c.modular.Divides(&s.values[1], &coefficient)
		if e != nil {
			return false, e
		}
		if !divides {
			return false, nil
		}
		if _, e = m.squareRootNumeral(4, &coefficient, &s.values[1], &s.values[2]); e != nil {
			return false, e
		}
		count, e := R.as.size(c)
		if e != nil {
			return false, e
		}
		for j := uint32(0); j < count; j++ {
			a, e := R.as.coefficient(c, j)
			if e != nil {
				return false, e
			}
			zero, e := c.modular.IsZero(&a)
			if e != nil {
				return false, e
			}
			if zero {
				continue
			}
			if _, e = m.squareRootNumeral(2, &a, &s.values[2], &s.values[3]); e != nil {
				return false, e
			}
			if _, e = m.squareRootNumeral(2, &s.values[3], &s.two, &s.values[3]); e != nil {
				return false, e
			}
			if _, e = m.squareRootNumeral(3, nil, nil, &s.values[3]); e != nil {
				return false, e
			}
			t, e := R.ms.monomial(c, j)
			if e != nil {
				return false, e
			}
			product, _, e := m.squareRootMonomial(2, t, mi)
			if e != nil {
				return false, e
			}
			if e = s.reference.assign(c, product); e != nil {
				return false, e
			}
			if e = m.squareRootAdd(C, &s.values[3], s.reference.value); e != nil {
				return false, e
			}
		}
		if _, e = m.squareRootNumeral(2, &s.values[2], &s.values[2], &s.values[3]); e != nil {
			return false, e
		}
		if _, e = m.squareRootNumeral(3, nil, nil, &s.values[3]); e != nil {
			return false, e
		}
		product, _, e := m.squareRootMonomial(2, mi, mi)
		if e != nil {
			return false, e
		}
		if e = s.reference.assign(c, product); e != nil {
			return false, e
		}
		if e = m.squareRootAdd(C, &s.values[3], s.reference.value); e != nil {
			return false, e
		}
		if e = m.squareRootAdd(R, &s.values[2], mi); e != nil {
			return false, e
		}
	}
}

// Explicit owning abort, independent of literal stopped source destruction.
func (m *MultivariatePolynomialManager) CleanupSqrt() error {
	return m.cleanupSqrtModeOwned(true, 0, &m.squareRoot.drain)
}
func (m *MultivariatePolynomialManager) cleanupSqrtModeOwned(abort bool, invocation uint64, cursor *uint32) error {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.squareRoot
	if !s.active && len(s.orphans) == 0 {
		return nil
	}
	if m.core == nil {
		return nil
	}
	c := m.core
	if s.executing {
		return ErrPolynomialPending
	}
	active, endEntered, _ := s.perfectSquare.State(c.integer)
	if active && (abort || endEntered) {
		if e := c.integer.CleanupPerfectSquareInvocation(&s.perfectSquare, true); e != nil {
			s.cleanupCause = e
			return e
		}
	}
	active, _, _ = s.perfectSquare.State(c.integer)
	if abort || !active {
		s.perfectSquare = unumeral.IntegerPerfectSquareInvocationReceipt{}
	}
	if e := c.integer.CleanupRawInvocation(&s.divisionRaw, false); e != nil {
		s.cleanupCause = e
		return e
	}
	if e := s.reference.drop(c, &s.reference.value, &s.reference.consumed, true); e != nil {
		s.cleanupCause = e
		return e
	}
	if abort {
		if e := s.reference.drop(c, &s.reference.incoming, &s.reference.incomingConsumed, true); e != nil {
			s.cleanupCause = e
			return e
		}
		s.reference.assigning = false
	}
	s.referenceCompleted = false
	for i := 3; i >= 0; i-- {
		if e := c.integer.Del(&s.values[i]); e != nil {
			s.cleanupCause = e
			return e
		}
	}
	s.mask = 0
	if !abort {
		return nil
	}
	if e := c.integer.Del(&s.two); e != nil {
		s.cleanupCause = e
		return e
	}
	if e := c.cleanupFactoryInvocation(&s.factory, true); e != nil {
		s.cleanupCause = e
		return e
	}
	if s.calleeGeneration != 0 {
		if c.monomials.creation.generation == s.calleeGeneration && c.monomials.creation.parent == c {
			if e := c.monomials.CleanupCreation(); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		if c.generatedCalleeGeneration == s.calleeGeneration {
			c.generatedCalleePending = false
		}
		s.calleeGeneration = 0
	}
	if e := c.monomials.cleanupMonomialAllocatorInvocation(&s.monomialAllocator, true); e != nil {
		s.cleanupCause = e
		return e
	}
	if e := m.cleanupSqrtOrphanInvocation(invocation, cursor); e != nil {
		return e
	}
	if e := c.integer.CleanupRawInvocation(&s.divisionRaw, true); e != nil {
		s.cleanupCause = e
		return e
	}
	if e := s.numeric.cleanup(c.integer); e != nil {
		s.cleanupCause = e
		return e
	}

	s.active = false
	s.cleanupCause = nil
	return nil
}
func (m *MultivariatePolynomialManager) closeSquareRootMetadata() error {
	if e := m.CleanupSqrt(); e != nil {
		return e
	}
	s := &m.squareRoot
	s.orphans = nil
	s.orphanInvocations = nil
	s.drain = 0
	return nil
}
