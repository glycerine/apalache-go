package polynomial

// Source: src/util/rational.h:54,67; src/util/mpq.h:186-189;
// src/math/polynomial/polynomial.cpp:2530-2536,2696-2698.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The source global rational binding is supplied as an actual borrowed owner.
// This is call-local source state embedded in the charged prerequisite header,
// not an imp-owned rational manager or an escaping per-call scope allocation.
type polynomialConstantScope struct {
	arithmeticScope                                                polynomialArithmeticScope
	rational                                                       unumeral.Rational
	numeral                                                        unumeral.Int
	result                                                         Polynomial
	active, rationalCompleted, numeralCompleted, failedConstructor bool
	cause, sourceCause                                             error
	stopped                                                        bool
	rationalStage                                                  uint8
}

func (m *MultivariatePolynomialCore) beginConstantScope() error {
	if m.config.Rationals == nil {
		return arena.ErrConfig
	}
	if e := m.config.Rationals.CheckOwnerReady(); e != nil {
		return e
	}
	if m.constantScope.active || m.constantScope.arithmeticScope.active || m.constantGeneration == ^uint64(0) {
		return ErrPolynomialPending
	}
	m.constantGeneration++
	if f := m.factorsOwner; f != nil && f.executing && m.callbackDepth == 0 {
		f.constantGeneration = m.constantGeneration
	}
	m.constantScope = polynomialConstantScope{active: true, rational: unumeral.NewRational()}
	if s := m.compositionOwner; s != nil && s.executing {
		s.constantGeneration = m.constantGeneration
	}
	return nil
}
func (m *MultivariatePolynomialCore) finishConstantScope(body error) error {
	s := &m.constantScope
	if s.cause == nil {
		s.cause = body
	}
	if s.stopped {
		return errors.Join(body, s.sourceCause)
	}
	// Only the actually acquired child of this real constant invocation may
	// stop the older NUM -> rational source suffix. A global normalization
	// history from another caller supplies no authority or stopped frontier.
	if s.arithmeticScope.active {

	}
	if s.numeralCompleted {
		if _, e := upolynomialSourceNumeralDel(m.integer, &s.numeral); e != nil {
			s.stopped = true
			s.sourceCause = e
			return errors.Join(body, e)
		}
		s.numeralCompleted = false
	}
	if s.rationalCompleted {
		roots := [2]*unumeral.Int{&(*s.rational.BorrowNumerator()), &(*s.rational.BorrowDenominator())}
		for s.rationalStage < 2 {
			if _, e := upolynomialSourceNumeralDel(m.integer, roots[s.rationalStage]); e != nil {
				s.stopped = true
				s.sourceCause = e
				return errors.Join(body, e)
			}
			s.rationalStage++
		}
		s.rationalCompleted = false
	}
	if s.failedConstructor {
		return body
	}
	if body == nil {
		s.active = false
	} else if s.result == (Polynomial{}) {
		s.active = false
	}
	if !s.active {
		*s = polynomialConstantScope{arithmeticScope: s.arithmeticScope}
	}
	return body
}
func (m *MultivariatePolynomialCore) MkConstantTakeCopy(a *unumeral.Int) (Polynomial, error) {
	return m.mkConstantTakeCopyWithArithmeticReceipt(a, nil)
}
func (m *MultivariatePolynomialCore) mkConstantTakeCopyWithArithmeticReceipt(a *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) (result Polynomial, cause error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if a == nil {
		return Polynomial{}, arena.ErrConfig
	}
	if e := m.beginConstantScope(); e != nil {
		return Polynomial{}, e
	}
	s := &m.constantScope
	if receipt != nil {
		if m.arithmeticInvocation == ^uint64(0) {
			return Polynomial{}, ErrPolynomialPending
		}
		m.arithmeticInvocation++
		s.arithmeticScope = polynomialArithmeticScope{active: true, executing: true, invocation: m.arithmeticInvocation, constantGeneration: m.constantGeneration}
		*receipt = polynomialArithmeticInvocationReceipt{manager: m, frame: &s.arithmeticScope, invocation: m.arithmeticInvocation}
		defer func() {
			s.arithmeticScope.executing = false
			s.arithmeticScope.bodyCause = cause
			if s.arithmeticScope.sourceCause == nil {
				s.arithmeticScope.sourceCause = s.sourceCause
			}
		}()
		if e := numericRationalManager_setIntegerWithArithmeticReceipt(m.config.Rationals, &s.rational, a, receipt); e != nil {
			s.failedConstructor = true
			s.cause = e
			return Polynomial{}, e
		}
		s.rationalCompleted = true
		p, e := m.constantFromRationalScopeWithArithmeticReceipt(receipt)
		if e == nil {
			s.arithmeticScope.active = false
		}
		return p, e
	}
	before, e := m.compositionNumericBefore()
	if e != nil {
		return Polynomial{}, e
	}
	e = m.compositionNumericAfter(before, m.config.Rationals.SetInteger(&s.rational, a))
	if e != nil {
		s.failedConstructor = true
		s.cause = e
		return Polynomial{}, e
	}
	s.rationalCompleted = true
	return m.constantFromRationalScope()
}
func (m *MultivariatePolynomialCore) constantFromRationalScope() (Polynomial, error) {
	return m.constantFromRationalScopeWithArithmeticReceipt(nil)
}
func (m *MultivariatePolynomialCore) constantFromRationalScopeWithArithmeticReceipt(receipt *polynomialArithmeticInvocationReceipt) (Polynomial, error) {
	s := &m.constantScope
	if e := m.cellSourceAssert(2531, "a.is_int()", func() (bool, error) { return unumeral.IsIntegerRational(&s.rational), nil }); e != nil {
		return Polynomial{}, m.finishConstantScope(e)
	}
	s.numeral = unumeral.Int{}
	s.numeralCompleted = true
	var copyCause error
	if receipt == nil {
		copyCause = m.compositionNumericCall(0, &(*s.rational.BorrowNumerator()), nil, nil, &s.numeral)
	} else {
		copyCause = numericIntegerModularManager_copyWithArithmeticReceipt(m.modular, &s.numeral, &(*s.rational.BorrowNumerator()), receipt)
	}
	if copyCause != nil {
		e := copyCause
		return Polynomial{}, m.finishConstantScope(e)
	}
	p, e := m.mkConstantTakeWithArithmeticReceipt(&s.numeral, receipt)
	if e != nil {
		return Polynomial{}, m.finishConstantScope(e)
	}
	s.result = p
	if receipt != nil && m.matchesFactoryInvocation(&receipt.frame.factory) {
		if e = m.transferFactoryResult(&receipt.frame.factory, p); e != nil {
			return Polynomial{}, m.finishConstantScope(e)
		}
	}
	if e = m.finishConstantScope(nil); e != nil {
		return Polynomial{}, e
	}
	return p, nil
}
func (m *MultivariatePolynomialCore) cleanupConstantScope() error {
	s := &m.constantScope
	if !s.active {
		return nil
	}
	if s.result != (Polynomial{}) && s.result != m.zero && s.result != m.unit {
		result := s.result
		e := m.CleanupValue(&s.result)
		// Placement returned one real polynomial, borrowed by the matching
		// operation receipt. Acknowledge its actual owning consumption here;
		// no directory/refcount inference or second deletion is performed.
		if s.result == (Polynomial{}) {
			if f := m.factorsOwner; f != nil && f.constantGeneration == m.constantGeneration && f.freshResult == result {
				f.freshResult = Polynomial{}
			}
			if c := m.compositionOwner; c != nil && c.constantGeneration == m.constantGeneration && c.freshResult == result {
				c.freshResult = Polynomial{}
			}
		}
		if e != nil {
			return e
		}
	}
	if s.numeralCompleted {
		if e := m.integer.Del(&s.numeral); e != nil {
			return e
		}
		s.numeralCompleted = false
	}
	// Explicit owning cleanup of the real fields of an incomplete rational
	// constructor, not execution of its nonexistent completed destructor.
	if e := m.config.Rationals.Del(&s.rational); e != nil {
		return e
	}
	cause, sourceCause, stopped := s.cause, s.sourceCause, s.stopped
	*s = polynomialConstantScope{cause: cause, sourceCause: sourceCause, stopped: stopped, arithmeticScope: s.arithmeticScope}
	return nil
}

// Completed source locals precede the parent's older declarations. Failed
// constructors and raw factory/singleton acquisitions are explicit abort only.
func (m *MultivariatePolynomialCore) cleanupConstantArithmeticInvocation(r *polynomialArithmeticInvocationReceipt, abort bool) error {
	s := &m.constantScope
	if r.frame != &s.arithmeticScope {
		return ErrPolynomialOwner
	}
	if s.numeralCompleted {
		if e := m.integer.Del(&s.numeral); e != nil {
			return e
		}
		s.numeralCompleted = false
	}
	if s.rationalCompleted {
		if e := m.config.Rationals.Del(&s.rational); e != nil {
			return e
		}
		s.rationalCompleted = false
	}
	if !abort {
		return nil
	}
	if e := m.cleanupConstantScope(); e != nil {
		return e
	}
	if e := m.cleanupFactoryInvocation(&r.frame.factory, true); e != nil {
		return e
	}
	if r.frame.singleton != (Monomial{}) && m.singletonReference && m.singletonMonomial == r.frame.singleton && m.singletonGeneration == r.frame.singletonGeneration {
		if e := m.monomials.DecRef(r.frame.singleton); e != nil {
			return e
		}
		m.singletonReference = false
		m.singletonMonomial = Monomial{}
		r.frame.singleton = Monomial{}
	}
	if e := r.frame.numeric.cleanup(m.integer); e != nil {
		return e
	}
	if e := m.modular.CleanupPNormalizeInvocation(&r.frame.normalize, true); e != nil {
		return e
	}
	r.frame.active = false
	return nil
}
