package polynomial

// Source: src/math/polynomial caller scopes: synchronous primitive borrows.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type borrowedPolynomialGuard interface {
	unumeral.NumericCallerGuard
	numericCallerOwner() any
	numericCallerBinding() *unumeral.NumericCallerBinding
}

func setBorrowedPolynomialCaller(z polynomialGuardOwner, owner any, kind uint32, q borrowedPolynomialGuard) {
	var current *unumeral.NumericCallerBinding
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == kind {
			if g, ok := b.Guard().(borrowedPolynomialGuard); ok {
				if g.numericCallerOwner() == owner {
					return
				}
				current = b
				break
			}
		}
	}
	if current != nil {
		var previous any
		for b := current.Previous(); b != nil; b = b.Previous() {
			if b.Kind() == kind {
				if g, ok := b.Guard().(borrowedPolynomialGuard); ok {
					previous = g.numericCallerOwner()
					break
				}
			}
		}
		if previous == owner {
			if e := current.Unbind(); e != nil {
				panic(e)
			}
			return
		}
	}
	if q == nil {
		if current == nil {
			return
		}
		panic(unumeral.ErrNumericOwner)
	}
	if e := z.BindCaller(q.numericCallerBinding(), q, kind); e != nil {
		panic(e)
	}
}
func algebraicNegCaller(z polynomialGuardOwner) *AlgebraicRootCore {
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == 8 {
			if q, ok := b.Guard().(borrowedPolynomialGuard); ok {
				if owner, ok := q.numericCallerOwner().(*AlgebraicRootCore); ok {
					return owner
				}
			}
		}
	}
	return nil
}
func setAlgebraicNegCaller(z polynomialGuardOwner, owner *AlgebraicRootCore) {
	var q borrowedPolynomialGuard
	var identity any
	if owner != nil {
		q = &owner.negation
		identity = owner
	}
	setBorrowedPolynomialCaller(z, identity, 8, q)
}
func (q *algebraicNegScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.negation && q.owner.negationExecuting()
}
func (q *algebraicNegScope) numericCallerOwner() any { return q.owner }
func (q *algebraicNegScope) numericCallerBinding() *unumeral.NumericCallerBinding {
	return &q.numeric.integerBinding
}
func factorCoreCaller(z polynomialGuardOwner) *polynomialFactorCoreFrame {
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == 9 {
			if q, ok := b.Guard().(borrowedPolynomialGuard); ok {
				if owner, ok := q.numericCallerOwner().(*polynomialFactorCoreFrame); ok {
					return owner
				}
			}
		}
	}
	return nil
}
func setFactorCoreCaller(z polynomialGuardOwner, owner *polynomialFactorCoreFrame) {
	var q borrowedPolynomialGuard
	var identity any
	if owner != nil {
		q = owner
		identity = owner
	}
	setBorrowedPolynomialCaller(z, identity, 9, q)
}
func (q *polynomialFactorCoreFrame) NumericExecuting() bool {
	return q != nil && q.manager != nil && q.manager.matchesFactorCore(&polynomialFactorCoreInvocationReceipt{q.manager, q, q.invocation}) && (q.executing || q.finishing)
}
func (q *polynomialFactorCoreFrame) numericCallerOwner() any { return q }
func (q *polynomialFactorCoreFrame) numericCallerBinding() *unumeral.NumericCallerBinding {
	return &q.callerBinding
}
func factorFrontendCaller(z polynomialGuardOwner) *upolyFactorFrontendScope {
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == 10 {
			if q, ok := b.Guard().(borrowedPolynomialGuard); ok {
				if owner, ok := q.numericCallerOwner().(*upolyFactorFrontendScope); ok {
					return owner
				}
			}
		}
	}
	return nil
}
func setFactorFrontendCaller(z polynomialGuardOwner, owner *upolyFactorFrontendScope) {
	var q borrowedPolynomialGuard
	var identity any
	if owner != nil {
		q = owner
		identity = owner
	}
	setBorrowedPolynomialCaller(z, identity, 10, q)
}
func (q *upolyFactorFrontendScope) NumericExecuting() bool {
	return q != nil && q.actual() && (q.executing || q.sourceExecuting || q.ownExecuting)
}
func (q *upolyFactorFrontendScope) numericCallerOwner() any { return q }
func (q *upolyFactorFrontendScope) numericCallerBinding() *unumeral.NumericCallerBinding {
	return &q.numeric.integerBinding
}
func upolyDisplayCaller(z polynomialGuardOwner) *upolyDisplayScope {
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == 11 {
			if q, ok := b.Guard().(borrowedPolynomialGuard); ok {
				if owner, ok := q.numericCallerOwner().(*upolyDisplayScope); ok {
					return owner
				}
			}
		}
	}
	return nil
}
func setUpolyDisplayCaller(z polynomialGuardOwner, owner *upolyDisplayScope) {
	var q borrowedPolynomialGuard
	var identity any
	if owner != nil {
		q = owner
		identity = owner
	}
	setBorrowedPolynomialCaller(z, identity, 11, q)
}
func (q *upolyDisplayScope) NumericExecuting() bool {
	return q != nil && q.actual() && (q.executing || q.sourceExecuting || q.ownExecuting)
}
func (q *upolyDisplayScope) numericCallerOwner() any { return q }
func (q *upolyDisplayScope) numericCallerBinding() *unumeral.NumericCallerBinding {
	return &q.numeric.integerBinding
}
func upolyDisplaySequenceCaller(z polynomialGuardOwner) *UPolynomialSequence {
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == 12 {
			if q, ok := b.Guard().(borrowedPolynomialGuard); ok {
				if owner, ok := q.numericCallerOwner().(*UPolynomialSequence); ok {
					return owner
				}
			}
		}
	}
	return nil
}
func setUpolyDisplaySequenceCaller(z polynomialGuardOwner, owner *UPolynomialSequence) {
	var q borrowedPolynomialGuard
	var identity any
	if owner != nil {
		q = owner
		identity = owner
	}
	setBorrowedPolynomialCaller(z, identity, 12, q)
}
func (q *UPolynomialSequence) NumericExecuting() bool {
	return q != nil && q.self == q && q.manager != nil && q.manager.self == q.manager && q.manager.displaySequence == q && q.displaying
}
func (q *UPolynomialSequence) numericCallerOwner() any { return q }
func (q *UPolynomialSequence) numericCallerBinding() *unumeral.NumericCallerBinding {
	return &q.callerBinding
}
func factorFrontendConstantCaller(z polynomialGuardOwner) *UPolynomialFactorizationService {
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == 13 {
			if q, ok := b.Guard().(borrowedPolynomialGuard); ok {
				if owner, ok := q.numericCallerOwner().(*UPolynomialFactorizationService); ok {
					return owner
				}
			}
		}
	}
	return nil
}
func setFactorFrontendConstantCaller(z polynomialGuardOwner, owner *UPolynomialFactorizationService) {
	var q borrowedPolynomialGuard
	var identity any
	if owner != nil {
		q = &owner.frontendConstant
		identity = owner
	}
	setBorrowedPolynomialCaller(z, identity, 13, q)
}
func (q *upolyFactorConstantBinding) NumericExecuting() bool {
	return q != nil && q.service != nil && q == &q.service.frontendConstant && q.service.constantCallerExecuting()
}
func (q *upolyFactorConstantBinding) numericCallerOwner() any { return q.service }
func (q *upolyFactorConstantBinding) numericCallerBinding() *unumeral.NumericCallerBinding {
	return &q.numeric.integerBinding
}
func factorUnivariateCaller(z polynomialGuardOwner) *MultivariatePolynomialManager {
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == 14 {
			if q, ok := b.Guard().(borrowedPolynomialGuard); ok {
				if owner, ok := q.numericCallerOwner().(*MultivariatePolynomialManager); ok {
					return owner
				}
			}
		}
	}
	return nil
}
func setFactorUnivariateCaller(z polynomialGuardOwner, owner *MultivariatePolynomialManager) {
	var q borrowedPolynomialGuard
	var identity any
	if owner != nil {
		q = &owner.factorUnivariate
		identity = owner
	}
	setBorrowedPolynomialCaller(z, identity, 14, q)
}
func (q *polynomialFactorUnivariateScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.factorUnivariate && q.invocation != 0 && (q.executing || q.finishing)
}
func (q *polynomialFactorUnivariateScope) numericCallerOwner() any { return q.owner }
func (q *polynomialFactorUnivariateScope) numericCallerBinding() *unumeral.NumericCallerBinding {
	return &q.callerBinding
}
func factorNumeralVectorCaller(z polynomialGuardOwner) *MultivariatePolynomialManager {
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == 15 {
			if q, ok := b.Guard().(borrowedPolynomialGuard); ok {
				if owner, ok := q.numericCallerOwner().(*MultivariatePolynomialManager); ok {
					return owner
				}
			}
		}
	}
	return nil
}
func setFactorNumeralVectorCaller(z polynomialGuardOwner, owner *MultivariatePolynomialManager) {
	var q borrowedPolynomialGuard
	var identity any
	if owner != nil {
		q = &owner.factorNumeralVector
		identity = owner
	}
	setBorrowedPolynomialCaller(z, identity, 15, q)
}
func (q *polynomialFactorNumeralVectorScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.factorNumeralVector && q.invocation != 0 && (q.executing || q.finishing)
}
func (q *polynomialFactorNumeralVectorScope) numericCallerOwner() any { return q.owner }
func (q *polynomialFactorNumeralVectorScope) numericCallerBinding() *unumeral.NumericCallerBinding {
	return &q.numeric.integerBinding
}
