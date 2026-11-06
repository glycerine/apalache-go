package polynomial

// Source: src/math/polynomial/polynomial.cpp:6832-6841.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialToPolynomialHistory struct {
	invocation                           uint64
	bodyCause, sourceCause, cleanupCause error
	normalize                            unumeral.IntegerNormalizeInvocationReceipt
	append                               polynomialCheapAppendInvocationReceipt
	factory                              polynomialFactoryInvocationReceipt
	monomialAllocator                    monomialAllocatorInvocationReceipt
}

func (m *MultivariatePolynomialManager) archiveToPolynomial() error {
	s := &m.toPolynomial
	if s.invocation == 0 {
		return nil
	}
	m.toPolynomialHistory = append(m.toPolynomialHistory, polynomialToPolynomialHistory{s.invocation, s.bodyCause, s.sourceCause, s.cleanupCause, s.normalize, s.append, s.factory, s.monomialAllocator})
	return nil
}
func (m *MultivariatePolynomialManager) closeToPolynomialHistory() {
	m.toPolynomialHistory = nil
}
func (m *MultivariatePolynomialManager) toPolynomialInvocationSourceCause(r *polynomialToPolynomialInvocationReceipt) error {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame != &m.toPolynomial || r.invocation == 0 {
		return nil
	}
	if r.invocation == m.toPolynomial.invocation {
		return m.toPolynomial.sourceCause
	}
	for i := range m.toPolynomialHistory {
		if m.toPolynomialHistory[i].invocation == r.invocation {
			return m.toPolynomialHistory[i].sourceCause
		}
	}
	return nil
}

type polynomialToPolynomialInvocationReceipt struct {
	manager    *MultivariatePolynomialManager
	frame      *polynomialToPolynomialScope
	invocation uint64
}

func (m *MultivariatePolynomialManager) matchesToPolynomialInvocation(r *polynomialToPolynomialInvocationReceipt) bool {
	return m != nil && m.self == m && m.core != nil && m.core.self == m.core && r != nil && r.manager == m && r.frame == &m.toPolynomial && r.invocation != 0 && r.invocation == r.frame.invocation
}
func numericIntegerManager_toPolynomialAllocationRoots(m *unumeral.IntegerManager, c integerAllocationInvocationContext) *polynomialNumericRoots {
	r := c.toPolynomial
	if m == nil || !m.ValidOwner() || r == nil || !r.manager.matchesToPolynomialInvocation(r) || c.toPolynomialInvocation != r.invocation || r.manager.core.integer != m || !r.frame.active || !r.frame.executing || r.manager.core.callbackDepth != 0 {
		return nil
	}
	return &r.frame.numeric
}
func numericScopedIntegerModularBuffer128_setToPolynomial(b *unumeral.ScopedIntegerModularBuffer128, i uint32, v *unumeral.Int, r *polynomialToPolynomialInvocationReceipt) error {
	if e := b.Check(); e != nil {
		return e
	}
	if r == nil || !r.manager.matchesToPolynomialInvocation(r) || b != &r.frame.buffer {
		return ErrPolynomialOwner
	}
	s := r.frame
	context := integerAllocationInvocationContext{toPolynomial: r, toPolynomialInvocation: r.invocation}
	if numericIntegerManager_toPolynomialAllocationRoots(b.Manager().Integers(), context) == nil || b.Manager() != r.manager.core.modular {
		return ErrPolynomialOwner
	}
	if b.SourceCause() != nil {
		return b.SourceCause()
	}
	if b.BodyCause() != nil {
		return b.BodyCause()
	}
	if s.transportBound {
		return ErrPolynomialPending
	}
	a, e := b.Get(i)
	if e != nil {
		return e
	}
	s.transport = a
	s.transportBound = true
	e = numericIntegerModularManager_copyWithToPolynomialReceipt(b.Manager(), &s.transport, v, r)
	write := b.Store(i, s.transport)
	if write == nil {
		s.transport = unumeral.Int{}
		s.transportBound = false
	}
	cause := polynomialNewtonJoin(e, write)
	if cause != nil && b.BodyCause() == nil {
		b.RecordBodyCause(cause)
	}
	return cause
}
func numericScopedIntegerModularBuffer128_pushToPolynomial(b *unumeral.ScopedIntegerModularBuffer128, v *unumeral.Int, r *polynomialToPolynomialInvocationReceipt) error {
	if e := b.Check(); e != nil {
		return e
	}
	if b.SourceCause() != nil {
		return b.SourceCause()
	}
	if b.BodyCause() != nil {
		return b.BodyCause()
	}
	if e := b.PushZero(); e != nil {
		b.RecordBodyCause(e)
		return e
	}
	return numericScopedIntegerModularBuffer128_setToPolynomial(b, b.Count()-1, v, r)
}
func (m *MultivariatePolynomialManager) cleanupToPolynomialInvocation(r *polynomialToPolynomialInvocationReceipt, abort bool) (cause error) {
	if !m.matchesToPolynomialInvocation(r) {
		return nil
	}
	s := r.frame
	if !s.active {
		return nil
	}
	if s.executing || s.finishing || m.core.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	s.finishing = true
	defer func() {
		s.finishing = false
		if cause != nil && s.cleanupCause == nil {
			s.cleanupCause = cause
		}
	}()
	if e := m.core.modular.CleanupPNormalizeInvocation(&s.normalize, false); e != nil {
		return e
	}
	if !abort {
		return nil
	}
	// Actual coefficient buffer is older than the returned factory, but the
	// unjoined factory has no source destructor. Abort it after buffer NUMs.
	if e := s.buffer.Cleanup(); e != nil {
		return e
	}
	if e := m.core.modular.Del(&s.transport); e != nil {
		return e
	}
	s.transportBound = false
	if e := m.core.modular.CleanupPNormalizeInvocation(&s.normalize, true); e != nil {
		return e
	}
	if e := s.numeric.cleanup(m.core.integer); e != nil {
		return e
	}
	if e := m.core.cleanupFactoryInvocation(&s.factory, true); e != nil {
		return e
	}
	if e := m.core.cheap.cleanupAppendInvocation(&s.append, true); e != nil {
		return e
	}
	if s.generated != (Monomial{}) {
		if e := m.core.monomials.CleanupValue(&s.generated); e != nil {
			return e
		}
	}
	if s.calleeGeneration != 0 && m.core.monomials.creation.toPolynomialParent == s && m.core.monomials.creation.toPolynomialInvocation == s.invocation {
		if e := m.core.monomials.CleanupCreation(); e != nil {
			return e
		}
		if m.core.generatedCalleeGeneration == s.calleeGeneration {
			m.core.generatedCalleePending = false
		}
	}
	if e := m.core.monomials.cleanupMonomialAllocatorInvocation(&s.monomialAllocator, true); e != nil {
		return e
	}
	s.active = false
	return nil
}

func numericIntegerModularManager_copyWithToPolynomialReceipt(m *unumeral.IntegerModularManager, out, value *unumeral.Int, r *polynomialToPolynomialInvocationReceipt) error {
	if e := m.Check(); e != nil {
		return e
	}
	context := integerAllocationInvocationContext{toPolynomial: r}
	if r != nil {
		context.toPolynomialInvocation = r.invocation
	}
	if r == nil || numericIntegerManager_toPolynomialAllocationRoots(m.Integers(), context) == nil || r.manager.core.modular != m {
		return ErrPolynomialOwner
	}
	if factorNumeralCallExecuting(m.Integers()) {
		return ErrPolynomialPending
	}
	if e := r.frame.numeric.operation.enter(m.Integers()); e != nil {
		return e
	}
	defer r.frame.numeric.operation.leave()
	return m.WithAllocationContext((context).utility(m.Integers())).Copy(out, value)
}

// Lifecycle exclusion only. Actual acquisition always uses the passed receipt.
func (m *MultivariatePolynomialCore) toPolynomialCallerExecuting() bool {
	if m == nil {
		return false
	}
	f := m.toPolynomialCaller
	return m != nil && m.self == m && f != nil && f.self == f && f.core == m && f.matchesToPolynomialInvocation(&f.toPolynomial.direct) && (f.toPolynomial.executing || f.toPolynomial.finishing)
}
