package polynomial

// Source: src/math/polynomial/polynomial.cpp:4600-4614; src/util/obj_ref.h.

import (
	"errors"
)

// Embedded actual result record is charged by the core's reflected header.
// It retains stopped source destruction and unjoined INC separately; no ticket.
type polynomialPowerScope struct {
	arithmetic              polynomialArithmeticInvocationReceipt
	executing               bool
	sourceCause             error
	generation              uint64
	pscOwner                uint64
	substitutionOwned       bool
	result                  polynomialReference
	active                  bool
	bodyCause, cleanupCause error
}

func (m *MultivariatePolynomialCore) Power(p Polynomial, k uint32, r *PolynomialRef) (err error) {
	if e := m.check(); e != nil {
		return e
	}
	if r == nil || r.owner != m {
		return ErrPolynomialOwner
	}
	if e := r.open(); e != nil {
		return e
	}
	return m.powerInto(p, k, &r.root)
}

// The same source engine accepts a completed default source reference.
func (m *MultivariatePolynomialCore) powerInto(p Polynomial, k uint32, r *polynomialReference) (err error) {
	return m.powerIntoWithReceipt(p, k, r, nil)
}
func (m *MultivariatePolynomialCore) powerIntoWithReceipt(p Polynomial, k uint32, r *polynomialReference, receipt *polynomialPowerInvocationReceipt) (err error) {
	if m == nil || m.self != m || r == nil {
		return ErrPolynomialOwner
	}
	if k == 0 {
		return r.assign(m, m.unit)
	}
	if k == 1 {
		return r.assign(m, p)
	}
	local := &m.powerLocal
	if local.active {
		return ErrPolynomialPending
	}
	if m.powerGeneration == ^uint64(0) {
		return ErrPolynomialPending
	}
	m.powerGeneration++
	*local = polynomialPowerScope{generation: m.powerGeneration, active: true, executing: true, substitutionOwned: m.substitution.scope.executing, pscOwner: m.pscExecutingOwner()}
	if receipt != nil {
		*receipt = polynomialPowerInvocationReceipt{m, local, local.generation}
	}
	if f := m.factorsOwner; f != nil && f.executing && m.callbackDepth == 0 {
		f.powerGeneration = local.generation
	}
	defer func() {
		local.executing = false
		local.bodyCause = err
		c := m.finishPolynomialPowerSource()
		if c != nil {
			err = errors.Join(err, c)
		}
	}()
	if e := local.result.assign(m, p); e != nil {
		return e
	}
	for i := uint32(1); i < k; i++ {
		product, e := m.mulWithReceipt(local.result.value, p, &local.arithmetic)
		if e != nil {
			return e
		}
		if e = local.result.assign(m, product); e != nil {
			return e
		}
	}
	return r.assign(m, local.result.value)
}

func (m *MultivariatePolynomialCore) finishPolynomialPowerSource() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	s := &m.powerLocal
	if s.executing {
		return ErrPolynomialPending
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if cause := m.arithmeticInvocationSourceCause(&s.arithmetic); cause != nil {
		s.sourceCause = cause
		s.cleanupCause = cause
		return cause
	}
	if e := m.cleanupArithmeticInvocation(&s.arithmetic, false); e != nil {
		s.sourceCause = e
		s.cleanupCause = e
		return e
	}
	if e := s.result.closeValue(m, false); e != nil {
		s.sourceCause = e
		s.cleanupCause = e
		return e
	}
	if s.result.value == (Polynomial{}) && s.result.incoming == (Polynomial{}) && !s.result.assigning && !m.matchesArithmeticInvocation(&s.arithmetic) {
		*s = polynomialPowerScope{generation: s.generation, sourceCause: s.sourceCause, bodyCause: s.bodyCause, cleanupCause: s.cleanupCause, arithmetic: s.arithmetic}
	}
	return nil
}

// Explicit owning abort follows all completed source destruction. Source
// Power never silently retries a stopped destructor or consumes unjoined INC.
func (m *MultivariatePolynomialCore) CleanupPolynomialPower() error {
	return m.cleanupPolynomialPowerPhase(true, true)
}
func (m *MultivariatePolynomialCore) cleanupPolynomialPowerPhase(owning, abort bool) error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.powerLocal.executing {
		return ErrPolynomialPending
	}
	if !owning {
		return m.finishPolynomialPowerSource()
	}
	if !m.powerLocal.active {
		return nil
	}
	if e := m.cleanupArithmeticInvocation(&m.powerLocal.arithmetic, false); e != nil {
		return e
	}
	if e := m.powerLocal.result.closeValue(m, owning); e != nil {
		m.powerLocal.cleanupCause = e
		return e
	}
	if abort {
		if e := m.powerLocal.result.abort(m); e != nil {
			m.powerLocal.cleanupCause = e
			return e
		}
	}
	if abort {
		if e := m.cleanupArithmeticInvocation(&m.powerLocal.arithmetic, true); e != nil {
			return e
		}
	}
	if m.powerLocal.result.assigning || m.powerLocal.result.incoming != (Polynomial{}) {
		return nil
	}
	if m.matchesArithmeticInvocation(&m.powerLocal.arithmetic) {
		return nil
	}
	m.powerLocal = polynomialPowerScope{generation: m.powerLocal.generation, sourceCause: m.powerLocal.sourceCause, bodyCause: m.powerLocal.bodyCause, cleanupCause: m.powerLocal.cleanupCause, arithmetic: m.powerLocal.arithmetic}
	return nil
}
