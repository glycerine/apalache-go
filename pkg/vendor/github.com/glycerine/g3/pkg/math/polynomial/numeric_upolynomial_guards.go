package polynomial

// Source: src/math/polynomial/upolynomial.cpp caller invocation ownership.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialGuardOwner interface {
	CallerBindings() *unumeral.NumericCallerBinding
	BindCaller(*unumeral.NumericCallerBinding, unumeral.NumericCallerGuard, uint32) error
}
type upolyNumericGuard interface {
	unumeral.NumericCallerGuard
	guardOwner() *UPolynomialManager
	guardRoots() *polynomialNumericRoots
}

func upolyCaller(z polynomialGuardOwner, kind uint32) *UPolynomialManager {
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == kind {
			if q, ok := b.Guard().(upolyNumericGuard); ok {
				return q.guardOwner()
			}
		}
	}
	return nil
}
func setUPolyCaller(z polynomialGuardOwner, owner *UPolynomialManager, kind uint32) {
	var current *unumeral.NumericCallerBinding
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == kind {
			if q, ok := b.Guard().(upolyNumericGuard); ok {
				if q.guardOwner() == owner {
					return
				}
				current = b
				break
			}
		}
	}
	if current != nil {
		if upolyPreviousCaller(current.Previous(), kind) == owner {
			if e := current.Unbind(); e != nil {
				panic(e)
			}
			return
		}
	}
	if owner == nil {
		if current == nil {
			return
		}
		panic(unumeral.ErrNumericOwner)
	}
	var q upolyNumericGuard
	switch kind {
	case 1:
		q = &owner.knuth
	case 2:
		q = &owner.rootUpperBound
	case 3:
		q = &owner.removeOneHalfRoot
	case 4:
		q = &owner.oneHalfRoot
	case 5:
		q = &owner.integerEvaluation
	case 6:
		q = &owner.rationalEvaluation
	case 7:
		q = &owner.binaryEvaluation
	default:
		panic(unumeral.ErrNumericOwner)
	}
	binding := &q.guardRoots().integerBinding
	if _, ok := z.(*unumeral.NaturalManager); ok {
		binding = &q.guardRoots().naturalBinding
	}
	if e := z.BindCaller(binding, q, kind); e != nil {
		panic(e)
	}
}
func upolyPreviousCaller(b *unumeral.NumericCallerBinding, kind uint32) *UPolynomialManager {
	for ; b != nil; b = b.Previous() {
		if b.Kind() == kind {
			if q, ok := b.Guard().(upolyNumericGuard); ok {
				return q.guardOwner()
			}
		}
	}
	return nil
}
func (q *upolyKnuthScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.knuth && q.owner.knuthExecuting()
}
func (q *upolyKnuthScope) guardOwner() *UPolynomialManager             { return q.owner }
func (q *upolyKnuthScope) guardRoots() *polynomialNumericRoots         { return &q.numeric }
func knuthCaller(z polynomialGuardOwner) *UPolynomialManager           { return upolyCaller(z, 1) }
func setKnuthCaller(z polynomialGuardOwner, owner *UPolynomialManager) { setUPolyCaller(z, owner, 1) }
func (q *upolyRootUpperBoundScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.rootUpperBound && q.owner.rootUpperBoundExecuting()
}
func (q *upolyRootUpperBoundScope) guardOwner() *UPolynomialManager     { return q.owner }
func (q *upolyRootUpperBoundScope) guardRoots() *polynomialNumericRoots { return &q.numeric }
func rootUpperBoundCaller(z polynomialGuardOwner) *UPolynomialManager   { return upolyCaller(z, 2) }
func setRootUpperBoundCaller(z polynomialGuardOwner, owner *UPolynomialManager) {
	setUPolyCaller(z, owner, 2)
}
func (q *upolyRemoveOneHalfRootScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.removeOneHalfRoot && q.owner.removeOneHalfRootExecuting()
}
func (q *upolyRemoveOneHalfRootScope) guardOwner() *UPolynomialManager     { return q.owner }
func (q *upolyRemoveOneHalfRootScope) guardRoots() *polynomialNumericRoots { return &q.numeric }
func removeOneHalfRootCaller(z polynomialGuardOwner) *UPolynomialManager   { return upolyCaller(z, 3) }
func setRemoveOneHalfRootCaller(z polynomialGuardOwner, owner *UPolynomialManager) {
	setUPolyCaller(z, owner, 3)
}
func (q *upolyOneHalfRootScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.oneHalfRoot && q.owner.oneHalfRootExecuting()
}
func (q *upolyOneHalfRootScope) guardOwner() *UPolynomialManager     { return q.owner }
func (q *upolyOneHalfRootScope) guardRoots() *polynomialNumericRoots { return &q.numeric }
func oneHalfRootCaller(z polynomialGuardOwner) *UPolynomialManager   { return upolyCaller(z, 4) }
func setOneHalfRootCaller(z polynomialGuardOwner, owner *UPolynomialManager) {
	setUPolyCaller(z, owner, 4)
}
func (q *upolyIntegerEvaluationScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.integerEvaluation && q.owner.integerEvaluationExecuting()
}
func (q *upolyIntegerEvaluationScope) guardOwner() *UPolynomialManager     { return q.owner }
func (q *upolyIntegerEvaluationScope) guardRoots() *polynomialNumericRoots { return &q.numeric }
func integerEvaluationCaller(z polynomialGuardOwner) *UPolynomialManager   { return upolyCaller(z, 5) }
func setIntegerEvaluationCaller(z polynomialGuardOwner, owner *UPolynomialManager) {
	setUPolyCaller(z, owner, 5)
}
func (q *upolyRationalEvaluationScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.rationalEvaluation && q.owner.rationalEvaluationExecuting()
}
func (q *upolyRationalEvaluationScope) guardOwner() *UPolynomialManager     { return q.owner }
func (q *upolyRationalEvaluationScope) guardRoots() *polynomialNumericRoots { return &q.numeric }
func rationalEvaluationCaller(z polynomialGuardOwner) *UPolynomialManager   { return upolyCaller(z, 6) }
func setRationalEvaluationCaller(z polynomialGuardOwner, owner *UPolynomialManager) {
	setUPolyCaller(z, owner, 6)
}
func (q *upolyBinaryEvaluationScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.binaryEvaluation && q.owner.binaryEvaluationExecuting()
}
func (q *upolyBinaryEvaluationScope) guardOwner() *UPolynomialManager     { return q.owner }
func (q *upolyBinaryEvaluationScope) guardRoots() *polynomialNumericRoots { return &q.numeric }
func binaryEvaluationCaller(z polynomialGuardOwner) *UPolynomialManager   { return upolyCaller(z, 7) }
func setBinaryEvaluationCaller(z polynomialGuardOwner, owner *UPolynomialManager) {
	setUPolyCaller(z, owner, 7)
}
