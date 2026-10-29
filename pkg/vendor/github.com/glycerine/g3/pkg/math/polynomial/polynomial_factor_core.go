package polynomial

// Source: src/math/polynomial/polynomial.cpp:6737-6830.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialFactorCoreInvocationReceipt struct {
	manager    *MultivariatePolynomialManager
	frame      *polynomialFactorCoreFrame
	invocation uint64
}
type polynomialFactorCoreFrame struct {
	childPrelude                                                 polynomialFactorDiagnosticScope
	callerBinding                                                unumeral.NumericCallerBinding
	self                                                         *polynomialFactorCoreFrame
	manager                                                      *MultivariatePolynomialManager
	previous                                                     *polynomialFactorCoreFrame
	target                                                       *PolynomialFactors
	invocation, header                                           uint64
	active, executing, finishing                                 bool
	lastCall                                                     uint8
	childEntering                                                bool
	numeralConstructed                                           bool
	references                                                   uint8
	sourceRef, ownRef                                            uint8
	sourceStarted, ownStarted, sourceNumeralDone, ownNumeralDone bool
	numeral                                                      unumeral.Int
	refs                                                         [7]polynomialReference // c,pp,C,C_prime,B,A,D
	minimum                                                      *ScopedPolynomialMaxDegrees
	child                                                        polynomialFactorCoreInvocationReceipt
	iccp                                                         polynomialICCPInvocationReceipt
	derivative                                                   polynomialDerivativeInvocationReceipt
	gcd                                                          polynomialGCDInvocationReceipt
	division                                                     polynomialExactDivInvocationReceipt
	constant                                                     polynomialFactorConstantInvocationReceipt
	univariate                                                   polynomialFactorUnivariateInvocationReceipt
	quadratic                                                    polynomialGCDInvocationReceipt
	traceDisplay                                                 polynomialDisplayInvocationReceipt
	traceNumeral                                                 unumeral.NumeralInsertionReceipt
	predicates, dispatchPredicates, helperPredicates             [2]polynomialGCDInvocationReceipt
	bodyCause, sourceCause, cleanupCause                         error
}

func (m *MultivariatePolynomialManager) matchesFactorCore(r *polynomialFactorCoreInvocationReceipt) bool {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame == nil || r.invocation == 0 {
		return false
	}
	for _, actual := range m.factorFrames {
		if actual == r.frame {
			return actual.self == actual && actual.manager == m && actual.invocation == r.invocation
		}
	}
	return false
}
func (m *MultivariatePolynomialManager) beginFactorCore(r *PolynomialFactors) (*polynomialFactorCoreFrame, error) {
	if m.factorCoreInvocation == ^uint64(0) {
		return nil, arena.ErrBounds
	}
	size := uint64(reflect.TypeOf(polynomialFactorCoreFrame{}).Size())
	if e := m.originalBudget.Reserve(size); e != nil {
		return nil, e
	}
	m.factorCoreInvocation++
	s := &polynomialFactorCoreFrame{manager: m, target: r, invocation: m.factorCoreInvocation, header: size, active: true, executing: true, previous: factorCoreCaller(m.core.integer)}
	s.self = s
	m.factorFrames = append(m.factorFrames, s)
	setFactorCoreCaller(m.core.integer, s)
	return s, nil
}
func (m *MultivariatePolynomialManager) factorCoreWithReceipt(p Polynomial, r *PolynomialFactors, params UPolynomialFactorParams, receipt *polynomialFactorCoreInvocationReceipt) (cause error) {
	return m.factorCoreSource(p, r, params, receipt, nil, nil)
}
func (m *MultivariatePolynomialManager) factorCoreSource(p Polynomial, r *PolynomialFactors, params UPolynomialFactorParams, receipt *polynomialFactorCoreInvocationReceipt, parent *polynomialFactorCoreFrame, accepted *polynomialFactorCoreInvocationReceipt) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if r == nil || r.manager != m {
		return ErrPolynomialOwner
	}
	if e := r.open(); e != nil {
		return e
	}
	if parent != nil {
		actual := polynomialFactorCoreInvocationReceipt{m, parent, parent.invocation}
		if !m.matchesFactorCore(&actual) || !parent.executing || parent.finishing || !parent.childEntering || receipt != &parent.child || parent.target != r || m.core.callbackDepth != 0 {
			return ErrPolynomialPending
		}
		parent.childEntering = false
	} else if m.factorCoreExecuting() {
		return ErrPolynomialPending
	}
	diagnostic := &m.factorPrelude
	if parent != nil {
		diagnostic = &parent.childPrelude
	}
	if e := m.traceFactorCoreEntry(p, r, diagnostic, parent); e != nil {
		return e
	}
	c := m.core
	if e := polynomialFactorSourceAssert(6757, "!is_zero(p)", func() (bool, error) {
		zero, e := c.IsZero(p)
		return !zero, e
	}); e != nil {
		return e
	}
	constant, e := c.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		if e = polynomialFactorSourceAssert(6759, "!is_zero(p)", func() (bool, error) {
			zero, e := c.IsZero(p)
			return !zero, e
		}); e != nil {
			return e
		}
		if e = polynomialFactorSourceAssert(6760, "p->size() == 1", func() (bool, error) {
			n, e := c.readWord(p, 8)
			return n == 1, e
		}); e != nil {
			return e
		}
		value, e := c.coefficientCore(p, 0)
		if e != nil {
			return e
		}
		if parent != nil {
			parent.lastCall = 1
			return m.accumulateFactorConstantWithReceipt(r, &value, &parent.constant)
		}
		return m.accumulateFactorConstantWithReceipt(r, &value, &m.factorInline)
	}
	s, e := m.beginFactorCore(r)
	if e != nil {
		return e
	}
	direct := polynomialFactorCoreInvocationReceipt{m, s, s.invocation}
	if receipt != nil {
		*receipt = direct
	}
	if accepted != nil {
		*accepted = direct
	}
	defer func() {
		s.executing = false
		if s.bodyCause == nil {
			s.bodyCause = cause
		}
		cause = errors.Join(cause, m.finishFactorCoreSource(&direct))
		if cause == nil {
			cause = m.cleanupFactorCoreInvocation(&direct, false)
			if cause == nil {
				cause = m.cleanupFactorCoreInvocation(&direct, true)
			}
		}
		setFactorCoreCaller(m.core.integer, s.previous)
		s.previous = nil
	}()
	x, minimum, e := m.minimumDegreeVariable(p)
	s.minimum = minimum
	if e != nil {
		return e
	}
	if e = polynomialFactorSourceAssert(6765, "degree(p, x) > 0", func() (bool, error) {
		d, e := c.Degree(p, x)
		return d > 0, e
	}); e != nil {
		return e
	}
	s.numeralConstructed = true
	s.references = 2
	s.lastCall = 2
	if e = m.iccpPartsWithReceipt(p, x, &s.numeral, &s.refs[0], &s.refs[1], &s.iccp); e != nil {
		return e
	}
	if e = m.traceFactorCoreInteger(s); e != nil {
		return e
	}
	s.lastCall = 1
	if e = m.accumulateFactorConstantWithReceipt(r, &s.numeral, &s.constant); e != nil {
		return e
	}
	s.lastCall = 3
	s.childEntering = true
	if e = m.factorCoreSource(s.refs[0].value, r, params, &s.child, s, nil); e != nil {
		s.childEntering = false
		return e
	}
	s.references = 3
	if e = s.refs[2].assign(c, s.refs[1].value); e != nil {
		return e
	}
	s.references = 4
	s.lastCall = 4
	d, e := m.derivativeWithReceipt(s.refs[2].value, x, &s.derivative)
	if e != nil {
		return e
	}
	if e = s.refs[3].assign(c, d); e != nil {
		return e
	}
	s.references = 7
	s.lastCall = 5
	if e = m.gcdIntoWithReceipt(s.refs[2].value, s.refs[3].value, &s.refs[4], &s.gcd); e != nil {
		return e
	}
	constant, e = c.IsConstant(s.refs[4].value)
	if e != nil {
		return e
	}
	if constant {
		if e = polynomialFactorSourceAssert(6782, "degree(C, x) > 0", func() (bool, error) {
			degree, e := c.Degree(s.refs[2].value, x)
			return degree > 0, e
		}); e != nil {
			return e
		}
		return m.factorSquareFreePrimitiveDispatch(s.refs[2].value, r, x, 1, params, s)
	}
	s.lastCall = 6
	a, e := c.exactDivWithReceipt(s.refs[2].value, s.refs[4].value, &s.division)
	if e != nil {
		return e
	}
	if e = s.refs[5].assign(c, a); e != nil {
		return e
	}
	j := uint32(1)
	for {
		constant, e = c.IsConstant(s.refs[5].value)
		if e != nil {
			return e
		}
		if constant {
			break
		}
		if e = polynomialFactorSourceAssert(6791, "is_primitive(A, x)", func() (bool, error) {
			s.lastCall = 7
			return m.isPrimitiveWithReceipt(s.refs[5].value, x, &s.predicates[0])
		}); e != nil {
			return e
		}
		if e = polynomialFactorSourceAssert(6792, "is_square_free(A, x)", func() (bool, error) {
			s.lastCall = 8
			return m.isSquareFreeWithReceipt(s.refs[5].value, x, true, &s.predicates[1])
		}); e != nil {
			return e
		}
		if e = polynomialFactorSourceAssert(6793, "degree(A, x) > 0", func() (bool, error) {
			degree, e := c.Degree(s.refs[5].value, x)
			return degree > 0, e
		}); e != nil {
			return e
		}
		if e = c.checkpoint(); e != nil {
			return e
		}
		if e = m.traceFactorCoreLoop(s, j); e != nil {
			return e
		}
		s.lastCall = 5
		if e = m.gcdIntoWithReceipt(s.refs[5].value, s.refs[4].value, &s.refs[6], &s.gcd); e != nil {
			return e
		}
		s.lastCall = 6
		result, e := c.exactDivWithReceipt(s.refs[5].value, s.refs[6].value, &s.division)
		if e != nil {
			return e
		}
		if e = s.refs[2].assign(c, result); e != nil {
			return e
		}
		constant, e = c.IsConstant(s.refs[2].value)
		if e != nil {
			return e
		}
		if !constant {
			if e = polynomialFactorSourceAssert(6803, "degree(C, x) > 0", func() (bool, error) {
				degree, e := c.Degree(s.refs[2].value, x)
				return degree > 0, e
			}); e != nil {
				return e
			}
			if e = m.factorSquareFreePrimitiveDispatch(s.refs[2].value, r, x, j, params, s); e != nil {
				return e
			}
		} else {
			if e = m.traceFactorCoreConstant(s); e != nil {
				return e
			}
			if e = polynomialFactorSourceAssert(6808, "C->size() == 1", func() (bool, error) {
				n, e := c.readWord(s.refs[2].value, 8)
				return n == 1, e
			}); e != nil {
				return e
			}
			if e = polynomialFactorSourceAssert(6809, "m_manager.is_one(C->a(0)) || m_manager.is_minus_one(C->a(0))", func() (bool, error) {
				coefficient, e := c.mutableCoefficientCore(s.refs[2].value, 0)
				if e != nil {
					return false, e
				}
				one, e := c.modular.IsOne(&coefficient)
				if e != nil || one {
					return one, e
				}
				coefficient, e = c.mutableCoefficientCore(s.refs[2].value, 0)
				if e != nil {
					return false, e
				}
				return c.modular.IsMinusOne(&coefficient)
			}); e != nil {
				return e
			}
			coefficient, e := c.mutableCoefficientCore(s.refs[2].value, 0)
			if e != nil {
				return e
			}
			minus, e := c.modular.IsMinusOne(&coefficient)
			if e != nil {
				return e
			}
			if minus && j%2 == 1 {
				s.lastCall = 1
				if e = m.flipFactorSignWithReceipt(r, &s.constant); e != nil {
					return e
				}
			}
		}
		s.lastCall = 6
		result, e = c.exactDivWithReceipt(s.refs[4].value, s.refs[6].value, &s.division)
		if e != nil {
			return e
		}
		if e = s.refs[4].assign(c, result); e != nil {
			return e
		}
		if e = s.refs[5].assign(c, s.refs[6].value); e != nil {
			return e
		}
		j++
	}
	if e = polynomialFactorSourceAssert(6819, "eq(mk_one(), A)", func() (bool, error) {
		return c.Equal(c.unit, s.refs[5].value)
	}); e != nil {
		return e
	}
	return nil
}
func (m *MultivariatePolynomialManager) factorSquareFreePrimitiveDispatch(p Polynomial, r *PolynomialFactors, x, k uint32, params UPolynomialFactorParams, s *polynomialFactorCoreFrame) error {
	c := m.core
	if e := polynomialFactorSourceAssert(6738, "degree(p, x) > 0", func() (bool, error) {
		degree, e := c.Degree(p, x)
		return degree > 0, e
	}); e != nil {
		return e
	}
	if e := polynomialFactorSourceAssert(6739, "is_primitive(p, x)", func() (bool, error) {
		s.lastCall = 9
		return m.isPrimitiveWithReceipt(p, x, &s.dispatchPredicates[0])
	}); e != nil {
		return e
	}
	if e := polynomialFactorSourceAssert(6740, "is_square_free(p, x)", func() (bool, error) {
		s.lastCall = 10
		return m.isSquareFreeWithReceipt(p, x, true, &s.dispatchPredicates[1])
	}); e != nil {
		return e
	}
	if e := polynomialFactorSourceAssert(6741, "!is_zero(p)", func() (bool, error) {
		zero, e := c.IsZero(p)
		return !zero, e
	}); e != nil {
		return e
	}
	degree, e := c.Degree(p, x)
	if e != nil {
		return e
	}
	if degree == 1 {
		return m.factorLinearSquareFreePrimitiveWithTraceReceipt(p, r, x, k, &s.helperPredicates, &s.lastCall, &s.quadratic)
	}
	univariate, e := c.IsUnivariate(p)
	if e != nil {
		return e
	}
	if univariate {
		s.lastCall = 11
		e = m.factorSquareFreePrimitiveUnivariateWithReceipt(p, r, k, params, &s.univariate)
		if e != nil {
			return e
		}
		if e = m.cleanupFactorUnivariateInvocation(&s.univariate, false); e != nil {
			return e
		}
		return m.cleanupFactorUnivariateInvocation(&s.univariate, true)
	}
	if degree == 2 {
		return m.factorQuadraticSquareFreePrimitiveWithReceipts(p, r, x, k, &s.helperPredicates, &s.quadratic, &s.lastCall)
	}
	return m.factorHigherSquareFreePrimitiveWithTraceReceipt(p, r, x, k, &s.helperPredicates, &s.lastCall, &s.quadratic)
}

func (m *MultivariatePolynomialManager) Factor(p Polynomial, r *PolynomialFactors, params UPolynomialFactorParams) error {
	return m.factorWithReceipt(p, r, params, nil)
}

// accepted is stamped only by an actually entered nonconstant source frame.
// Constant children belong to the concrete factors owner supplied by the caller.
func (m *MultivariatePolynomialManager) factorWithReceipt(p Polynomial, r *PolynomialFactors, params UPolynomialFactorParams, accepted *polynomialFactorCoreInvocationReceipt) error {
	if e := m.ready(); e != nil {
		return e
	}
	if r == nil || r.manager != m {
		return ErrPolynomialOwner
	}
	if e := r.open(); e != nil {
		return e
	}
	if m.factorCoreExecuting() {
		return ErrPolynomialPending
	}
	zero, e := m.core.IsZero(p)
	if e != nil {
		return e
	}
	if zero {
		value := unumeral.NewInt(0)
		return r.setConstantWithReceipt(&value, &m.factorInline)
	}
	if m.matchesFactorCore(&m.factorRoot) && m.factorRoot.frame.active {
		constant, e := m.core.IsConstant(p)
		if e != nil {
			return e
		}
		if !constant {
			return ErrPolynomialPending
		}
	}
	if e = m.factorCoreSource(p, r, params, &m.factorRoot, nil, accepted); e != nil {
		return e
	}
	return m.traceFactorEnd(r)
}
func (m *MultivariatePolynomialManager) factorCoreChildSource(s *polynomialFactorCoreFrame) error {
	if s.childPrelude.sourceCause != nil {
		return s.childPrelude.sourceCause
	}
	if e := s.traceNumeral.SourceCause(); e != nil {
		return e
	}
	if e := m.core.displayReceiptSourceCause(&s.traceDisplay); e != nil {
		return e
	}
	if e := m.factorLastChildSource(s); e != nil {
		return e
	}
	if e := m.gcdInvocationSourceCause(&s.quadratic); e != nil {
		return e
	}
	for i := 1; i >= 0; i-- {
		if e := m.gcdInvocationSourceCause(&s.helperPredicates[i]); e != nil {
			return e
		}
	}
	if m.matchesFactorUnivariate(&s.univariate) && s.univariate.frame.sourceCause != nil {
		return s.univariate.frame.sourceCause
	}
	for i := 1; i >= 0; i-- {
		if e := m.gcdInvocationSourceCause(&s.dispatchPredicates[i]); e != nil {
			return e
		}
	}
	if e := m.core.exactDivInvocationSourceCause(&s.division); e != nil {
		return e
	}
	if e := m.gcdInvocationSourceCause(&s.gcd); e != nil {
		return e
	}
	for i := 1; i >= 0; i-- {
		if e := m.gcdInvocationSourceCause(&s.predicates[i]); e != nil {
			return e
		}
	}
	if e := m.derivativeInvocationSourceCause(&s.derivative); e != nil {
		return e
	}
	if m.matchesFactorCore(&s.child) && s.child.frame.sourceCause != nil {
		return s.child.frame.sourceCause
	}
	if s.constant.manager != nil {
		if e := s.constant.manager.factorConstantInvocationSourceCause(&s.constant); e != nil {
			return e
		}
	}
	return m.iccpInvocationSourceCause(&s.iccp)
}
func (m *MultivariatePolynomialManager) finishFactorCoreSource(r *polynomialFactorCoreInvocationReceipt) (cause error) {
	if !m.matchesFactorCore(r) {
		return ErrPolynomialOwner
	}
	s := r.frame
	if s.executing || s.finishing || m.core.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	s.finishing = true
	defer func() {
		s.finishing = false
		if cause != nil && s.sourceCause == nil {
			s.sourceCause = cause
		}
	}()
	if e := m.factorCoreChildSource(s); e != nil {
		return e
	}
	if !s.sourceStarted {
		s.sourceStarted = true
		s.sourceRef = s.references
	}
	for s.sourceRef > 0 {
		if e := s.refs[s.sourceRef-1].closeValue(m.core, false); e != nil {
			return e
		}
		s.sourceRef--
	}
	if s.numeralConstructed && !s.sourceNumeralDone {
		if _, e := upolynomialSourceNumeralDel(m.core.integer, &s.numeral); e != nil {
			return e
		}
		s.sourceNumeralDone = true
	}
	return nil
}
func (m *MultivariatePolynomialManager) cleanupFactorCoreChildren(s *polynomialFactorCoreFrame, abort bool) error {
	if e := m.cleanupFactorDiagnostic(&s.childPrelude); e != nil {
		return e
	}
	if s.traceNumeral.Active() {
		if e := s.traceNumeral.Complete(m.core.integer.Natural, false); e != nil {
			return e
		}
		if e := s.traceNumeral.Complete(m.core.integer.Natural, true); e != nil {
			return e
		}
	}
	if e := m.core.cleanupDisplayReceipt(&s.traceDisplay); e != nil {
		return e
	}
	if e := m.cleanupFactorLastChild(s, abort); e != nil {
		return e
	}
	if e := m.cleanupGCDInvocation(&s.quadratic, abort); e != nil {
		return e
	}
	for i := 1; i >= 0; i-- {
		if e := m.cleanupGCDInvocation(&s.helperPredicates[i], abort); e != nil {
			return e
		}
	}
	if e := m.cleanupFactorUnivariateInvocation(&s.univariate, abort); e != nil {
		return e
	}
	for i := 1; i >= 0; i-- {
		if e := m.cleanupGCDInvocation(&s.dispatchPredicates[i], abort); e != nil {
			return e
		}
	}
	if e := m.core.cleanupExactDivInvocation(&s.division, abort); e != nil {
		return e
	}
	if e := m.cleanupGCDInvocation(&s.gcd, abort); e != nil {
		return e
	}
	for i := 1; i >= 0; i-- {
		if e := m.cleanupGCDInvocation(&s.predicates[i], abort); e != nil {
			return e
		}
	}
	if e := m.cleanupDerivativeInvocation(&s.derivative, abort); e != nil {
		return e
	}
	if e := m.cleanupFactorCoreInvocation(&s.child, abort); e != nil {
		return e
	}
	if s.constant.manager != nil {
		if e := s.constant.manager.cleanupFactorConstantInvocation(&s.constant, abort); e != nil {
			return e
		}
	}
	return m.cleanupICCPInvocation(&s.iccp, abort)
}
func (m *MultivariatePolynomialManager) cleanupFactorCoreInvocation(r *polynomialFactorCoreInvocationReceipt, abort bool) (cause error) {
	if !m.matchesFactorCore(r) {
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
	if e := m.cleanupFactorCoreChildren(s, false); e != nil {
		return e
	}
	if !s.ownStarted {
		s.ownStarted = true
		s.ownRef = s.references
	}
	for s.ownRef > 0 {
		if e := s.refs[s.ownRef-1].close(m.core); e != nil {
			return e
		}
		s.ownRef--
	}
	if s.numeralConstructed && !s.ownNumeralDone {
		if e := m.core.integer.CleanupRawNumeral(&s.numeral); e != nil {
			return e
		}
		s.ownNumeralDone = true
	}
	if s.minimum != nil {
		if e := s.minimum.CloseChecked(); e != nil {
			return e
		}
	}
	if !abort {
		return nil
	}
	if e := m.cleanupFactorCoreChildren(s, true); e != nil {
		return e
	}
	s.active = false
	return nil
}
func (m *MultivariatePolynomialManager) CleanupFactor() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.core.evaluationTraceCallback() {
		return ErrPolynomialPending
	}
	if e := m.cleanupFactorDiagnostic(&m.factorEnd); e != nil {
		return e
	}
	if e := m.cleanupFactorDiagnostic(&m.factorPrelude); e != nil {
		return e
	}
	if r := &m.factorInline; r.manager != nil {
		if e := r.manager.cleanupFactorConstantInvocation(r, false); e != nil {
			return e
		}
	}
	if e := m.cleanupFactorCoreInvocation(&m.factorRoot, false); e != nil {
		return e
	}
	if e := m.cleanupFactorCoreInvocation(&m.factorRoot, true); e != nil {
		return e
	}
	if r := &m.factorInline; r.manager != nil {
		return r.manager.cleanupFactorConstantInvocation(r, true)
	}
	return nil
}
func numericIntegerManager_factorCoreCallerExecuting(m *unumeral.IntegerManager) bool {
	if m == nil || !m.ValidOwner() {
		return false
	}
	for s := factorCoreCaller(m); s != nil; s = s.previous {
		r := polynomialFactorCoreInvocationReceipt{s.manager, s, s.invocation}
		if s.manager.matchesFactorCore(&r) && s.manager.core.integer == m && (s.executing || s.finishing) {
			return true
		}
	}
	return false
}
func (m *MultivariatePolynomialManager) closeFactorCoreHistory() {
	for _, s := range m.factorFrames {
		m.originalBudget.Release(s.header)
	}
	m.factorFrames = nil
}

func (m *MultivariatePolynomialManager) factorLastChildSource(s *polynomialFactorCoreFrame) error {
	switch s.lastCall {
	case 1:
		if s.constant.manager != nil {
			return s.constant.manager.factorConstantInvocationSourceCause(&s.constant)
		}
	case 2:
		return m.iccpInvocationSourceCause(&s.iccp)
	case 3:
		if m.matchesFactorCore(&s.child) {
			return s.child.frame.sourceCause
		}
	case 4:
		return m.derivativeInvocationSourceCause(&s.derivative)
	case 5:
		return m.gcdInvocationSourceCause(&s.gcd)
	case 6:
		return m.core.exactDivInvocationSourceCause(&s.division)
	case 7, 8:
		return m.gcdInvocationSourceCause(&s.predicates[s.lastCall-7])
	case 9, 10:
		return m.gcdInvocationSourceCause(&s.dispatchPredicates[s.lastCall-9])
	case 11:
		if m.matchesFactorUnivariate(&s.univariate) {
			return s.univariate.frame.sourceCause
		}
	case 12:
		return m.gcdInvocationSourceCause(&s.quadratic)
	case 13, 14:
		return m.gcdInvocationSourceCause(&s.helperPredicates[s.lastCall-13])
	}
	return nil
}
func (m *MultivariatePolynomialManager) cleanupFactorLastChild(s *polynomialFactorCoreFrame, abort bool) error {
	switch s.lastCall {
	case 1:
		if s.constant.manager != nil {
			return s.constant.manager.cleanupFactorConstantInvocation(&s.constant, abort)
		}
	case 2:
		return m.cleanupICCPInvocation(&s.iccp, abort)
	case 3:
		return m.cleanupFactorCoreInvocation(&s.child, abort)
	case 4:
		return m.cleanupDerivativeInvocation(&s.derivative, abort)
	case 5:
		return m.cleanupGCDInvocation(&s.gcd, abort)
	case 6:
		return m.core.cleanupExactDivInvocation(&s.division, abort)
	case 7, 8:
		return m.cleanupGCDInvocation(&s.predicates[s.lastCall-7], abort)
	case 9, 10:
		return m.cleanupGCDInvocation(&s.dispatchPredicates[s.lastCall-9], abort)
	case 11:
		return m.cleanupFactorUnivariateInvocation(&s.univariate, abort)
	case 12:
		return m.cleanupGCDInvocation(&s.quadratic, abort)
	case 13, 14:
		return m.cleanupGCDInvocation(&s.helperPredicates[s.lastCall-13], abort)
	}
	return nil
}

func (m *MultivariatePolynomialManager) factorCoreExecuting() bool {
	if m == nil || m.self != m {
		return false
	}
	if r := &m.factorInline; r.manager != nil && r.manager.matchesFactorConstantInvocation(r) && (r.frame.executing || r.frame.finishing) {
		return true
	}
	for _, q := range m.factorFrames {
		if q != nil && q.self == q && q.manager == m && (q.executing || q.finishing) {
			return true
		}
	}
	return false
}
