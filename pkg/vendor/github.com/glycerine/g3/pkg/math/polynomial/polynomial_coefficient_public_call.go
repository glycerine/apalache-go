package polynomial

// Source: src/math/polynomial/polynomial.cpp:3264-3277,3297-3312,7034-7040;
// src/util/obj_ref.h. Retained nlsat_explain.cpp:335 caller assignment.

import "errors"

// PolynomialCoefficientCall records the exact children accepted by one
// coeff-with-reduct expression and its subsequent owning assignment. It does
// not own either caller reference. Keep it until those caller locals have been
// destroyed and CleanupRetained has acknowledged any interrupted children.
// The zero value is ready for use; shallow copies are invalid after first use.
type PolynomialCoefficientCall struct {
	self              *PolynomialCoefficientCall
	owner             *MultivariatePolynomialCore
	coefficient       polynomialCoefficientInvocationReceipt
	append            polynomialCheapAppendInvocationReceipt
	factory           polynomialFactoryInvocationReceipt
	active, executing bool
}

// CoeffWithReductInto follows lc = coeff(p,x,k,reduct): reduct is assigned by
// the callee before the returned coefficient is acquired by lc. INC precedes
// the old lc DEC, including when the input aliases either caller reference.
func (m *MultivariatePolynomialCore) CoeffWithReductInto(p Polynomial, x, k uint32, lc, reduct *PolynomialRef, call *PolynomialCoefficientCall) error {
	if reduct == nil {
		return ErrPolynomialOwner
	}
	return m.coefficientInto(p, x, k, lc, reduct, call)
}

// CoeffInto is the retained lc = coeff(p,x,k) overload. It does not construct
// or assign a reduct, and preserves that overload's actual source trace path.
func (m *MultivariatePolynomialCore) CoeffInto(p Polynomial, x, k uint32, lc *PolynomialRef, call *PolynomialCoefficientCall) error {
	return m.coefficientInto(p, x, k, lc, nil, call)
}
func (m *MultivariatePolynomialCore) coefficientInto(p Polynomial, x, k uint32, lc, reduct *PolynomialRef, call *PolynomialCoefficientCall) (err error) {
	if err = m.check(); err != nil {
		return err
	}
	if call == nil || (call.self != nil && call.self != call) || lc == nil || lc.owner != m || (reduct != nil && reduct.owner != m) {
		return ErrPolynomialOwner
	}
	if call.active || call.executing {
		return ErrPolynomialPending
	}
	if err = lc.open(); err != nil {
		return err
	}
	if reduct != nil {
		if err = reduct.open(); err != nil {
			return err
		}
	}
	if lc.root.assigning || (reduct != nil && reduct.root.assigning) {
		return ErrPolynomialPending
	}
	*call = PolynomialCoefficientCall{self: call, owner: m, active: true, executing: true}
	defer func() {
		call.executing = false
		err = errors.Join(err, call.FinishSource())
	}()
	var rest *polynomialReference
	if reduct != nil {
		rest = &reduct.root
	}
	result, err := m.coeffCoreWithReceipts(p, x, k, rest, &call.coefficient, &call.append, &call.factory)
	if err != nil {
		return err
	}
	return lc.root.assignWithFactoryReceipt(m, result, &call.factory)
}

// FinishSource performs only accepted callee lexical cleanup. The caller's
// lc/reduct destructors must run before aborting unjoined child allocations.
func (call *PolynomialCoefficientCall) FinishSource() error {
	if call == nil || call.self == nil {
		return nil
	}
	if call.self != call || call.owner == nil {
		return ErrPolynomialOwner
	}
	if call.executing {
		return ErrPolynomialPending
	}
	call.executing = true
	defer func() { call.executing = false }()
	m := call.owner
	if err := m.cleanupCoefficientInvocation(&call.coefficient, false); err != nil {
		return err
	}
	if !m.matchesCoefficientInvocation(&call.coefficient) && !m.matchesFactoryInvocation(&call.factory) && !m.cheap.matchesAppendInvocation(&call.append) {
		call.active = false
	}
	return nil
}

// CleanupRetained retires only this expression's accepted coefficient,
// factory and append invocations. It never closes the borrowed caller refs or
// adopts an older/newer invocation that happened to use the same buffers.
func (call *PolynomialCoefficientCall) CleanupRetained() error {
	if err := call.FinishSource(); err != nil {
		return err
	}
	if call == nil || call.self == nil {
		return nil
	}
	call.executing = true
	defer func() { call.executing = false }()
	m := call.owner
	if err := m.cleanupCoefficientInvocation(&call.coefficient, true); err != nil {
		return err
	}
	if err := m.cleanupFactoryInvocation(&call.factory, true); err != nil {
		return err
	}
	if err := m.cheap.cleanupAppendInvocation(&call.append, true); err != nil {
		return err
	}
	call.active = false
	return nil
}
