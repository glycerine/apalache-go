package polynomial

// Source: src/math/polynomial/polynomial.cpp:3264-3277,3297-3312,1392-1423.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialCoefficientScope struct {
	bufferActive                          bool
	arithmetic                            [2]polynomialArithmeticInvocationReceipt
	sourceCause                           error
	invocation, calleeGeneration          uint64
	active, executing, foreignTransferred bool
	generatedCleanupStarted               bool
	generated                             Monomial
	allocator                             [3]umemory.Block
}
type polynomialCoefficientInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialCoefficientScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesCoefficientInvocation(r *polynomialCoefficientInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.coefficientInvocation && r.invocation != 0 && r.frame.invocation == r.invocation && r.frame.active
}
func (m *MultivariatePolynomialCore) transferCoefficientGenerated(r *polynomialCoefficientInvocationReceipt, term Monomial) {
	s := &m.coefficientInvocation
	if !s.active || s.generated == (Monomial{}) || s.generated != term {
		return
	}
	if m.matchesCoefficientInvocation(r) && s.executing {
		s.generated = Monomial{}
	} else {
		// This real INC belongs to a foreign append, so ownership cannot be
		// recovered merely by finding the same monomial token in that vector.
		s.foreignTransferred = true
	}
}
func (m *MultivariatePolynomialCore) cleanupCoefficientInvocation(r *polynomialCoefficientInvocationReceipt, abort bool) error {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame != &m.coefficientInvocation || r.invocation == 0 || r.invocation != r.frame.invocation {
		return nil
	}
	s := r.frame
	if s.executing || s.foreignTransferred {
		return ErrPolynomialPending
	}
	for i := len(s.arithmetic) - 1; i >= 0; i-- {
		if e := m.cleanupArithmeticInvocation(&s.arithmetic[i], false); e != nil {
			return e
		}
	}
	if !abort {
		return nil
	}
	for i := len(s.arithmetic) - 1; i >= 0; i-- {
		if e := m.cleanupArithmeticInvocation(&s.arithmetic[i], true); e != nil {
			return e
		}
	}
	if s.generated != (Monomial{}) {
		// A caller INC is a refusal guard, never authority for this root.
		if !s.generatedCleanupStarted {
			count, e := m.monomialRefCount(s.generated)
			if e != nil {
				return e
			}
			if count != 0 {
				return ErrPolynomialPending
			}
			s.generatedCleanupStarted = true
		}
		before := s.generated
		e := m.monomials.CleanupValue(&s.generated)
		if s.generated == (Monomial{}) && m.generatedMonomial == before {
			m.generatedMonomial = Monomial{}
			m.generatedActive = false
			m.generatedPSC = 0
			m.generatedBuffer = nil
		}
		if e != nil {
			return e
		}
	}
	if s.calleeGeneration != 0 && m.monomials.creation.generation == s.calleeGeneration && m.monomials.creation.coefficientParent == s && m.monomials.creation.coefficientInvocation == s.invocation {
		if e := m.monomials.CleanupCreation(); e != nil {
			return e
		}
	}
	// Only actual newly acquired allocator leaf tokens belong to this invocation.
	roots, e := m.monomials.allocator.PendingRoots()
	if e != nil {
		return e
	}
	owned, foreign := false, false
	for i := range roots {
		if roots[i] != (umemory.Block{}) {
			if roots[i] == s.allocator[i] {
				owned = true
			} else {
				foreign = true
			}
		}
	}
	if owned && foreign {
		return ErrPolynomialPending
	}
	if owned {
		e = m.monomials.allocator.CleanupPending()
		now, x := m.monomials.allocator.PendingRoots()
		if x != nil {
			return polynomialNewtonJoin(e, x)
		}
		s.allocator = now
		if e != nil {
			return e
		}
	}
	*s = polynomialCoefficientScope{invocation: s.invocation, sourceCause: s.sourceCause, arithmetic: s.arithmetic}
	return nil
}

// A coefficient invocation owns the two actual source buffer children only at
// reached ADD and mk actions; this contains no mirrored coefficient payload.
func (m *MultivariatePolynomialCore) coefficientSourceAdd(b *polynomialSOM, a *unumeral.Int, term Monomial, scope *polynomialCoefficientScope, r *polynomialCoefficientInvocationReceipt, appendReceipt *polynomialCheapAppendInvocationReceipt, id int) error {
	if scope == nil {
		return b.addConsumedCoefficientReceipts(a, term, false, nil, nil, appendReceipt, r)
	}
	ar := &scope.arithmetic[id]
	if ar.invocation == 0 {
		if e := b.acquireArithmeticInvocationWithCoefficient(ar, r); e != nil {
			return e
		}
	}
	e := b.addConsumedScalarDivEngine(a, term, false, nil, nil, nil, r, nil, ar)
	if appendReceipt != nil && ar.frame.append.invocation != 0 {
		*appendReceipt = ar.frame.append
	}
	return e
}
func (m *MultivariatePolynomialCore) coefficientSourceMK(b *polynomialSOM, scope *polynomialCoefficientScope, appendReceipt *polynomialCheapAppendInvocationReceipt, factoryReceipt *polynomialFactoryInvocationReceipt, id int) (Polynomial, error) {
	if scope == nil {
		return b.mkWithAppendReceipt(false, appendReceipt, factoryReceipt)
	}
	ar := &scope.arithmetic[id]
	if ar.invocation == 0 {
		parent := polynomialCoefficientInvocationReceipt{manager: m, frame: scope, invocation: scope.invocation}
		if e := b.acquireArithmeticInvocationWithCoefficient(ar, &parent); e != nil {
			return Polynomial{}, e
		}
	}
	ar.frame.executing = true
	p, e := b.mkWithAppendReceipt(false, &ar.frame.append, &ar.frame.factory)
	ar.frame.executing = false
	if e == nil {
		e = m.cleanupArithmeticInvocation(ar, false)
	}
	if e == nil {
		ar.frame.coefficientReady = true
	}
	// Copy only the concrete identifier stamped at the actual constructor leaf.
	// It describes no physical payload and cannot name a later factory.
	if factoryReceipt != nil && ar.frame.factory.invocation != 0 {
		*factoryReceipt = ar.frame.factory
	}
	return p, e
}
func (m *MultivariatePolynomialCore) coefficientInvocationSourceCause(r *polynomialCoefficientInvocationReceipt) error {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame != &m.coefficientInvocation || r.invocation == 0 || r.invocation != r.frame.invocation {
		return nil
	}
	s := r.frame
	if s.sourceCause != nil {
		return s.sourceCause
	}
	for i := len(s.arithmetic) - 1; i >= 0; i-- {
		if e := m.arithmeticInvocationSourceCause(&s.arithmetic[i]); e != nil {
			s.sourceCause = e
			return e
		}
	}
	return nil
}

func (m *MultivariatePolynomialCore) coefficientAssignReduct(r *polynomialReference, p Polynomial, s *polynomialCoefficientScope) error {
	if s == nil {
		return r.assign(m, p)
	}
	ar := &s.arithmetic[1]
	return r.assignWithFactoryReceipt(m, p, &ar.frame.factory)
}

// Only the real factory handoff can complete this child's adapter metadata.
// No payload is deleted and no shared vector action is manufactured here.
func (m *MultivariatePolynomialCore) acknowledgeCoefficientArithmeticFactory(s *polynomialFactoryScope, r *polynomialFactoryInvocationReceipt) {
	if !m.matchesFactoryInvocation(r) || s != &m.factoryInvocation || r.frame != s {
		return
	}
	b := s.buffer
	if !m.ownsArithmeticBuffer(b) {
		return
	}
	ar := &b.arithmeticScope
	parent := ar.coefficientOwner
	if parent != &m.coefficientInvocation || ar.coefficientInvocation == 0 || ar.coefficientInvocation != parent.invocation || !ar.coefficientReady || !ar.active || ar.executing || ar.factory.manager != m || ar.factory.frame != r.frame || ar.factory.invocation != r.invocation {
		return
	}
	captured := false
	for i := range parent.arithmetic {
		child := &parent.arithmetic[i]
		if child.manager == m && child.frame == ar && child.invocation == ar.invocation {
			captured = true
		}
	}
	if !captured {
		return
	}
	ar.active = false
	for i := range parent.arithmetic {
		child := &parent.arithmetic[i]
		if m.matchesArithmeticInvocation(child) {
			return
		}
	}
	parent.bufferActive = false
}
