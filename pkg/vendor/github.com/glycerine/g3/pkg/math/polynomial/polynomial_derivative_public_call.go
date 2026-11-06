package polynomial

// Source: src/math/polynomial/polynomial.cpp:4507-4528,7228-7229;
// src/nlsat/nlsat_explain.cpp:679. Concrete retained owning assignment.

import "errors"

// PolynomialDerivativeCall names the actual derivative invocation accepted by
// one caller expression. The caller continues to own its output reference.
// A zero value is ready; copying an initialized call is invalid.
type PolynomialDerivativeCall struct {
	self              *PolynomialDerivativeCall
	owner             *MultivariatePolynomialManager
	receipt           polynomialDerivativeInvocationReceipt
	active, executing bool
}

func (call *PolynomialDerivativeCall) pending() bool {
	m := call.owner
	if m.matchesDerivativeInvocation(&call.receipt) {
		return true
	}
	for i, invocation := range m.derivative.orphanInvocations {
		if call.receipt.invocation != 0 && invocation == call.receipt.invocation && m.derivative.orphans[i] != (Monomial{}) {
			return true
		}
	}
	return false
}

// DerivativeInto preserves p_prime = derivative(p,x): the callee's lexical
// numerals finish before the returned pointer INC and old-output DEC.
func (m *MultivariatePolynomialManager) DerivativeInto(p Polynomial, x uint32, out *PolynomialRef, call *PolynomialDerivativeCall) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if call == nil || (call.self != nil && call.self != call) || out == nil || out.owner != m.core {
		return ErrPolynomialOwner
	}
	if call.active || call.executing {
		return ErrPolynomialPending
	}
	if err = out.open(); err != nil {
		return err
	}
	if out.root.assigning {
		return ErrPolynomialPending
	}
	*call = PolynomialDerivativeCall{self: call, owner: m, active: true, executing: true}
	defer func() { call.executing = false; err = errors.Join(err, call.FinishSource()) }()
	result, err := m.derivativeWithReceipt(p, x, &call.receipt)
	if err != nil {
		return err
	}
	return out.root.assign(m.core, result)
}
func (call *PolynomialDerivativeCall) check() error {
	if call == nil || call.self == nil {
		return nil
	}
	if call.self != call || call.owner == nil {
		return ErrPolynomialOwner
	}
	if call.executing {
		return ErrPolynomialPending
	}
	return nil
}

// FinishSource acknowledges only accepted source numeral destruction. It does
// not retry a stopped destructor or retire unpublished monomial allocations.
func (call *PolynomialDerivativeCall) FinishSource() error {
	if err := call.check(); err != nil {
		return err
	}
	if call == nil || call.self == nil {
		return nil
	}
	call.executing = true
	defer func() { call.executing = false }()
	if call.owner.matchesDerivativeInvocation(&call.receipt) && call.receipt.frame.stopped {
		return call.receipt.frame.sourceCause
	}
	// derivativeWithReceipt already attempted every lexical NUM destructor.
	// Observing completion must not read/destroy those ended NUM lifetimes again.
	call.active = call.pending()
	return nil
}

// FinishRetainedLocals resumes only this callee's lexical numerals. Its
// unjoined buffers/allocations remain until the older caller locals die.
func (call *PolynomialDerivativeCall) FinishRetainedLocals() error {
	if err := call.check(); err != nil {
		return err
	}
	if call == nil || call.self == nil {
		return nil
	}
	call.executing = true
	defer func() { call.executing = false }()
	return call.owner.cleanupDerivativeInvocation(&call.receipt, false)
}

// CleanupRetained is the explicit owning continuation after the caller locals
// finish. Matching invocation cohorts protect older/newer independent calls.
func (call *PolynomialDerivativeCall) CleanupRetained() error {
	if err := call.check(); err != nil {
		return err
	}
	if call == nil || call.self == nil {
		return nil
	}
	call.executing = true
	defer func() { call.executing = false }()
	if err := call.owner.cleanupDerivativeInvocation(&call.receipt, true); err != nil {
		return err
	}
	call.active = call.pending()
	return nil
}
