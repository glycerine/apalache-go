package polynomial

// Source: src/math/polynomial/polynomial.cpp:3335-3338,7042-7044.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// One actual scoped_numeral and the exact accepted const_coeff invocation.
// Native continuation metadata is embedded in the real core owner.
type polynomialNonzeroConstantScope struct {
	invocation                            uint64
	active, executing, numeralConstructed bool
	value                                 unumeral.Int
	constant                              polynomialConstantCoefficientInvocationReceipt
	bodyCause, sourceCause, cleanupCause  error
}

func (m *MultivariatePolynomialCore) nonzeroConstantPending() bool {
	return m != nil && m.self == m && m.nonzeroConstant.active
}

// PolynomialNonzeroConstantCall names only an actually accepted predicate call.
// A refused call does not change it or acquire another invocation's cleanup.
type PolynomialNonzeroConstantCall struct {
	owner      *MultivariatePolynomialCore
	invocation uint64
}

func (call *PolynomialNonzeroConstantCall) CleanupRetained() error {
	if call == nil || call.owner == nil {
		return nil
	}
	m := call.owner
	if m.self != m {
		return ErrPolynomialOwner
	}
	if !m.nonzeroConstant.active || m.nonzeroConstant.invocation != call.invocation {
		return nil
	}
	return m.CleanupNonzeroConstantCoefficient()
}

func (m *MultivariatePolynomialCore) NonzeroConstantCoefficient(p Polynomial, x, k uint32) (bool, error) {
	return m.nonzeroConstantCoefficientWithCall(p, x, k, nil)
}

// NonzeroConstantCoefficientWithCall records the invocation at actual entry,
// before the callee can refuse or acquire numeral storage.
func (m *MultivariatePolynomialCore) NonzeroConstantCoefficientWithCall(p Polynomial, x, k uint32, call *PolynomialNonzeroConstantCall) (bool, error) {
	return m.nonzeroConstantCoefficientWithCall(p, x, k, call)
}
func (m *MultivariatePolynomialCore) nonzeroConstantCoefficientWithCall(p Polynomial, x, k uint32, call *PolynomialNonzeroConstantCall) (result bool, err error) {
	if e := m.check(); e != nil {
		return false, e
	}
	s := &m.nonzeroConstant
	if s.active {
		return false, ErrPolynomialPending
	}
	if s.invocation == ^uint64(0) {
		return false, arena.ErrBounds
	}
	*s = polynomialNonzeroConstantScope{invocation: s.invocation + 1, active: true, executing: true, numeralConstructed: true}
	if call != nil {
		*call = PolynomialNonzeroConstantCall{owner: m, invocation: s.invocation}
	}
	defer func() {
		s.executing = false
		s.bodyCause = err
		err = errors.Join(err, m.finishNonzeroConstantSource())
	}()
	constant, e := m.constantCoefficientWithReceipt(p, x, k, &s.value, &s.constant)
	if e != nil || !constant {
		return false, e
	}
	zero, e := m.modular.IsZero(&s.value)
	return !zero, e
}

func (m *MultivariatePolynomialCore) finishNonzeroConstantSource() (err error) {
	s := &m.nonzeroConstant
	if !s.active {
		return nil
	}
	if s.executing {
		return ErrPolynomialPending
	}
	defer func() {
		if err != nil && s.sourceCause == nil {
			s.sourceCause = err
		}
	}()
	// A callee's source lexical destruction precedes our older numeral.
	if err = m.cleanupConstantCoefficientInvocation(&s.constant, false); err != nil {
		return err
	}
	if s.numeralConstructed {
		if err = m.deleteNumeral(&s.value); err != nil {
			return err
		}
		s.numeralConstructed = false
	}
	if s.bodyCause == nil && !m.matchesConstantCoefficientInvocation(&s.constant) {
		s.active = false
	}
	return nil
}

// CleanupNonzeroConstantCoefficient retires only this predicate's numeral and
// actually accepted child receipt. It never acknowledges an independent copy
// or a different constant-coefficient invocation after this receipt went stale.
func (m *MultivariatePolynomialCore) CleanupNonzeroConstantCoefficient() (err error) {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	s := &m.nonzeroConstant
	if !s.active {
		return nil
	}
	if s.executing {
		return ErrPolynomialPending
	}
	defer func() {
		if err != nil && s.cleanupCause == nil {
			s.cleanupCause = err
		}
	}()
	if err = m.cleanupConstantCoefficientInvocation(&s.constant, false); err != nil {
		return err
	}
	if s.numeralConstructed {
		if err = m.deleteNumeral(&s.value); err != nil {
			return err
		}
		s.numeralConstructed = false
	}
	if err = m.cleanupConstantCoefficientInvocation(&s.constant, true); err != nil {
		return err
	}
	*s = polynomialNonzeroConstantScope{invocation: s.invocation, constant: s.constant, bodyCause: s.bodyCause, sourceCause: s.sourceCause, cleanupCause: s.cleanupCause}
	return nil
}
