package polynomial

// Source: src/math/polynomial/upolynomial.cpp:3042-3051;
// src/util/mpz.cpp:560-584; src/util/mpzzp.h:58-68.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// The early SET has no source NUM/vector declaration. Its embedded binding
// costs no per-call allocation. Only a reached numeric allocation issues a
// charged stable call frame; inline SET never archives or replaces old roots.

type upolyFactorConstantBinding struct {
	service    *UPolynomialFactorizationService
	invocation uint64
	executing  bool
	result     *UPolynomialFactors
	record     *upolyFactorConstantCall
	normalize  unumeral.IntegerNormalizeInvocationReceipt
	numeric    polynomialNumericRoots // remains empty until an allocating record exists
}
type upolyFactorConstantCall struct {
	result                           *UPolynomialFactors
	self                             *upolyFactorConstantCall
	service                          *UPolynomialFactorizationService
	next                             *upolyFactorConstantCall
	header, invocation               uint64
	numeric                          polynomialNumericRoots
	normalize                        unumeral.IntegerNormalizeInvocationReceipt
	bodyCause, sourceCause, ownCause error
	ownExecuting, retired            bool
}

func (c integerAllocationInvocationContext) factorFrontendConstantFrame(z *unumeral.IntegerManager) *upolyFactorConstantBinding {
	b := c.factorFrontendConstant
	if z == nil || !z.ValidOwner() || b == nil || b.service == nil {
		return nil
	}
	s := b.service
	if s.self != s || s.closed || b != &s.frontendConstant || !b.executing || b.invocation == 0 || b.invocation != c.factorFrontendConstantInvocation || s.manager == nil || s.manager.self != s.manager || s.manager.config.Integer != z || s.manager.callbackDepth != 0 {
		return nil
	}
	if q := b.record; q != nil && (q.self != q || q.service != s || q.header == 0 || q.invocation != b.invocation || q.retired || !s.ownsFrontendConstantCall(q)) {
		return nil
	}
	return b
}
func (s *UPolynomialFactorizationService) ownsFrontendConstantCall(q *upolyFactorConstantCall) bool {
	for old := s.frontendConstantCalls; old != nil; old = old.next {
		if old == q {
			return true
		}
	}
	return false
}
func (c integerAllocationInvocationContext) prepareFactorFrontendConstantHistory(z *unumeral.IntegerManager) error {
	b := c.factorFrontendConstantFrame(z)
	if b == nil || b.record != nil {
		return nil
	}
	bytes := uint64(reflect.TypeOf(upolyFactorConstantCall{}).Size())
	if e := z.Budget().Reserve(bytes); e != nil {
		return e
	}
	q := &upolyFactorConstantCall{service: b.service, next: b.service.frontendConstantCalls, header: bytes, invocation: b.invocation, normalize: b.normalize, result: b.result}
	q.self = q
	b.record = q
	b.service.frontendConstantCalls = q
	return nil
}
func (s *UPolynomialFactorizationService) setFrontendConstant(fs *UPolynomialFactors, a *unumeral.Int) (cause error) {
	b := &s.frontendConstant
	if b.executing || b.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	*b = upolyFactorConstantBinding{service: s, invocation: b.invocation + 1, executing: true, result: fs}
	m, z := s.manager, s.manager.config.Integer
	previousM, previousZ := m.factorFrontendConstantCaller, factorFrontendConstantCaller(z)
	m.factorFrontendConstantCaller = s
	setFactorFrontendConstantCaller(z, s)
	defer func() {
		b.executing = false
		m.factorFrontendConstantCaller = previousM
		setFactorFrontendConstantCaller(z, previousZ)
		if q := b.record; q != nil {
			q.normalize = b.normalize
			q.bodyCause = cause
			q.sourceCause = nil
			q.retired = !polynomialNegNumericPending(&q.numeric) && !m.modular.MatchesPNormalizeInvocation(&q.normalize)
		}
	}()
	context := integerAllocationInvocationContext{factorFrontendConstant: b, factorFrontendConstantInvocation: b.invocation}
	return fs.setConstantWithSourceContext(a, context)
}
func (s *UPolynomialFactorizationService) constantCallerExecuting() bool {
	if s == nil || s.self != s || s.closed || s.manager == nil || s.manager.self != s.manager {
		return false
	}
	if b := &s.frontendConstant; b.service == s && b.invocation != 0 && b.executing {
		return true
	}
	q := s.frontendConstantOwn
	return q != nil && q.self == q && q.service == s && q.header != 0 && q.ownExecuting
}
func (s *UPolynomialFactorizationService) cleanupFrontendConstantCalls() error {
	if s.constantCallerExecuting() {
		return ErrUPolynomialCleanupPending
	}
	for q := s.frontendConstantCalls; q != nil; q = q.next {
		if q.retired {
			continue
		}
		if e := s.cleanupFrontendConstantCall(q); e != nil {
			return e
		}
	}
	return nil
}
func (s *UPolynomialFactorizationService) cleanupFrontendConstantCall(q *upolyFactorConstantCall) (cause error) {
	if q == nil || q.self != q || q.service != s || q.header == 0 || q.invocation == 0 || !s.ownsFrontendConstantCall(q) {
		return ErrUPolynomialOwner
	}
	if s.constantCallerExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if q.retired {
		return nil
	}
	m, z := s.manager, s.manager.config.Integer
	q.ownExecuting = true
	s.frontendConstantOwn = q
	previousM, previousZ := m.factorFrontendConstantCaller, factorFrontendConstantCaller(z)
	m.factorFrontendConstantCaller = s
	setFactorFrontendConstantCaller(z, s)
	defer func() {
		q.ownExecuting = false
		s.frontendConstantOwn = nil
		m.factorFrontendConstantCaller = previousM
		setFactorFrontendConstantCaller(z, previousZ)
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, false); e != nil {
		return e
	}
	// There are no source automatic caller locals on the erased branch.
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
		return e
	}
	if e := cleanupUPolynomialNumericRoots(z, &q.numeric); e != nil {
		return e
	}
	q.retired = true
	return nil
}
