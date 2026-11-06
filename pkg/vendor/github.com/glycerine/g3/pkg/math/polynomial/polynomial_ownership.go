package polynomial

// Source: src/math/polynomial/polynomial.cpp:1392-1423,2481-2495.
// Explicit owning abort for retained incomplete placement; not a C++ destructor
// of an object whose constructor did not complete.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func (m *MultivariatePolynomialCore) monomialRefCount(a Monomial) (uint32, error) {
	l, e := m.monomials.location(a)
	if e != nil {
		return 0, e
	}
	return m.monomials.allocator.ReadUint32(l.raw, 0)
}
func (m *MultivariatePolynomialCore) cleanupCreation() error {
	if _, e := m.factoryCallerGuard(); e != nil {
		return e
	}
	if e := m.cleanupGeneratedMonomial(); e != nil {
		return e
	}
	if e := m.cleanupLinearGenerated(); e != nil {
		return e
	}
	if m.generatedCalleePending {
		e := m.monomials.CleanupCreation()
		if m.monomials.creation.stage == 0 {
			m.generatedCalleePending = false
		}
		if e != nil {
			return e
		}
	}
	if e := m.cleanupPendingCreation(); e != nil {
		return e
	}
	// The actual singleton local acquired a reference before placement. Its
	// unconsumed reference is a named owning-abort root, not a source destructor.
	if m.singletonReference {
		if e := m.monomials.DecRef(m.singletonMonomial); e != nil {
			return e
		}
		if m.pscReceipts.singleton == m.singletonMonomial {
			m.pscReceipts.singleton = Monomial{}
		}
		m.singletonReference = false
		m.singletonMonomial = Monomial{}
	}
	if s := &m.factoryInvocation; s.active {
		r := polynomialFactoryInvocationReceipt{manager: m, frame: s, invocation: s.invocation}
		if e := m.cleanupFactoryInvocation(&r, true); e != nil {
			return e
		}
	}
	return nil
}

// Drain only the actual pending placement, excluding other lexical owners.
func (m *MultivariatePolynomialCore) cleanupPendingCreation() error {
	p := &m.pending
	// The real source move clears its vector slot before writing placement.
	// Only that constructor's exact append frame may retain the intervening NUM.
	if s := p.appendParent; s != nil && s == &m.cheap.appendScope && s.active && s.factoryInvocation == p.generation {
		if e := m.deleteNumeral(&s.movingCoefficient); e != nil {
			return e
		}
	}
	if p.value != (Polynomial{}) {
		l, e := m.polynomialLocation(p.value)
		if e != nil {
			return e
		}
		for l.deleteIndex < l.initializedCoefficients || l.deleteIndex < l.initializedMonomials {
			i := l.deleteIndex
			if i < l.initializedCoefficients {
				a, e := m.coefficientFieldCore(p.value, i)
				if e != nil {
					return e
				}
				e = m.deleteNumeral(&a)
				if w := m.writeCoefficient(p.value, i, a); w != nil {
					return w
				}
				if e != nil {
					return e
				}
			}
			if i < l.initializedMonomials {
				a, e := m.monomialFieldCore(p.value, i)
				if e != nil {
					return e
				}
				if e = m.monomials.DecRef(a); e != nil {
					return e
				}
			}
			l.deleteIndex++
		}
	}
	if p.idConsumed {
		if !m.idsSourceDestroyed {
			if e := m.recycleID(p.id); e != nil {
				return e
			}
		}
		p.idConsumed = false
	}
	if p.value != (Polynomial{}) {
		l, e := m.polynomialLocation(p.value)
		if e != nil {
			return e
		}
		if !l.released {
			status, e := m.monomials.allocator.DeallocateTracked(uint64(l.requested), l.raw)
			if status.PhysicalReleased || status.PooledLinked {
				l.released = true
			}
			if e != nil {
				return e
			}
		}
		l.live = false
	} else if p.raw != (umemory.ObjectRef{}) {
		status, e := m.monomials.allocator.DeallocateTracked(uint64(p.requested), p.raw)
		if status.PhysicalReleased || status.PooledLinked {
			p.raw = umemory.ObjectRef{}
		}
		if e != nil {
			return e
		}
	}
	*p = polynomialCreation{}
	return nil
}
func (m *MultivariatePolynomialCore) CleanupConstruction() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.coefficientInvocation.active {
		return ErrPolynomialPending
	}
	return m.cleanupCreation()
}

// A newly interned, returned source local has not acquired a reference yet.
// This is owning abort only; existing interned results remain borrowed even
// with RC zero. Authority comes from the callee's new-publication branch.
func (m *MultivariatePolynomialCore) cleanupGeneratedMonomial() error {
	if !m.generatedActive {
		return nil
	}
	if m.generatedBuffer != nil {
		m.generatedBuffer.unreferencedConsumed = true
		m.generatedBuffer.unreferencedIndex = m.generatedIndex
	}
	if e := m.monomials.CleanupValue(&m.generatedMonomial); e != nil {
		return e
	}
	m.generatedActive = false
	m.generatedPSC = 0
	m.generatedBuffer = nil
	return nil
}
func (m *MultivariatePolynomialCore) generatedVariable(v, k uint32) (Monomial, error) {
	return m.generatedVariableWithOutcome(v, k, nil)
}
func (m *MultivariatePolynomialCore) generatedVariableWithOutcome(v, k uint32, result *monomialFactoryOutcome) (Monomial, error) {
	if m.generatedActive {
		return Monomial{}, ErrPolynomialPending
	}
	outcome := monomialFactoryOutcome{}
	a, e := m.monomials.mkVariableCreated(v, k, &outcome)
	if result != nil {
		*result = outcome
	}
	if e != nil && outcome.started {
		m.recordGeneratedCallee(outcome)
	}
	if e == nil && outcome.created {
		m.generatedMonomial = a
		if f := m.factorsOwner; f != nil && f.executing && m.callbackDepth == 0 {
			f.generated = a
		}
		if s := m.compositionOwner; s != nil && s.executing {
			s.coreGenerated = a
		}
		m.generatedActive = true
		m.generatedPSC = m.pscExecutingOwner()
		if m.translation.executing {
			m.translation.generated = a
		}
		if m.substitution.scope.executing {
			m.substitution.generatedReceipt = a
		}
	}
	return a, e
}
func (m *MultivariatePolynomialCore) generatedProduct(a, b Monomial) (Monomial, error) {
	return m.generatedProductWithOutcome(a, b, nil)
}
func (m *MultivariatePolynomialCore) generatedProductWithOutcome(a, b Monomial, result *monomialFactoryOutcome) (Monomial, error) {
	if m.generatedActive {
		return Monomial{}, ErrPolynomialPending
	}
	outcome := monomialFactoryOutcome{}
	r, e := m.monomials.mulCreated(a, b, &outcome)
	if result != nil {
		*result = outcome
	}
	if e != nil && outcome.started {
		m.recordGeneratedCallee(outcome)
	}
	if e == nil && outcome.created {
		m.generatedMonomial = r
		if f := m.factorsOwner; f != nil && f.executing && m.callbackDepth == 0 {
			f.generated = r
		}
		if s := m.compositionOwner; s != nil && s.executing {
			s.coreGenerated = r
		}
		m.generatedActive = true
		m.generatedPSC = m.pscExecutingOwner()
		if m.translation.executing {
			m.translation.generated = r
		}
		if m.substitution.scope.executing {
			m.substitution.generatedReceipt = r
		}
	}
	return r, e
}
func (m *MultivariatePolynomialCore) transferGeneratedMonomial() {
	if f := m.factorsOwner; f != nil && f.generated == m.generatedMonomial {
		f.generated = Monomial{}
	}
	if s := m.compositionOwner; s != nil && s.coreGenerated == m.generatedMonomial {
		s.coreGenerated = Monomial{}
	}
	if m.translation.generated == m.generatedMonomial {
		m.translation.generated = Monomial{}
	}
	if m.substitution.generatedReceipt == m.generatedMonomial {
		m.substitution.generatedReceipt = Monomial{}
	}
	m.generatedMonomial = Monomial{}
	m.generatedActive = false
	m.generatedPSC = 0
	m.generatedBuffer = nil
}

// Native tags retain cleanup authority for actual generated monomial slots.
func (m *MultivariatePolynomialCore) prepareLinearGenerated(i uint32) {
	for uint64(len(m.linearGenerated)) <= uint64(i) {
		m.linearGenerated = append(m.linearGenerated, false)
	}
	m.linearGenerated[i] = false
	m.linearGeneratedCleanup = 0
}
func (m *MultivariatePolynomialCore) cleanupLinearGenerated() error {
	for m.linearGeneratedCleanup < uint32(len(m.linearGenerated)) {
		i := m.linearGeneratedCleanup
		if m.linearGenerated[i] {
			if uint64(i) >= uint64(len(m.linearMonomials)) {
				return arena.ErrBounds
			}
			a := &m.linearMonomials[i]
			e := m.monomials.CleanupValue(a)
			if *a == (Monomial{}) {
				m.linearGenerated[i] = false
			}
			if e != nil {
				return e
			}
		}
		m.linearGeneratedCleanup++
	}
	return nil
}

// Record only the current actual factory constructor's callee authority.
func (m *MultivariatePolynomialCore) recordGeneratedCallee(outcome monomialFactoryOutcome) {
	c := &m.monomials.creation
	if !outcome.started || outcome.generation == 0 || c.generation != outcome.generation {
		return
	}
	if f := m.factorsOwner; f != nil && f.executing && m.callbackDepth == 0 {
		f.calleeGeneration = outcome.generation
	}
	m.generatedCalleePending = true
	m.generatedCalleeGeneration = outcome.generation
	if s := m.compositionOwner; s != nil && s.executing {
		s.calleeGeneration = outcome.generation
	}
	if m.translation.executing {
		m.translation.factoryGeneration = outcome.generation
	}
	c.parent = m
	if m.pscExecutingOwner() != 0 {
		m.pscReceipts.factory = outcome.generation
	}
	if m.substitution.scope.executing {
		m.substitution.generatedCalleeReceipt = outcome.generation
	}
}
