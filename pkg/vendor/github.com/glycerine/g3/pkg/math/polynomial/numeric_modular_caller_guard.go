package polynomial

// Source: src/math/polynomial caller reentry guards.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func numericIntegerModularManager_upolynomialCallerExecuting(m *unumeral.IntegerModularManager) bool {
	if m == nil || !m.ValidOwner() || m.Integers() == nil || !m.Integers().ValidOwner() {
		return false
	}
	z := m.Integers()
	for binding := z.CallerBindings(); binding != nil; binding = binding.Previous() {
		switch binding.Kind() {
		case 20:
			if q, ok := binding.Guard().(*sexprPolynomialArithmeticScope); ok && q.NumericExecuting() && q.owner.polynomial.modular == m {
				return true
			}
		case 17:
			if q, ok := binding.Guard().(*upolySequenceSturmScope); ok && q.NumericExecuting() && q.owner.manager.modular == m {
				return true
			}
		case 18:
			if q, ok := binding.Guard().(*upolySturmIsolationScope); ok && q.NumericExecuting() && q.owner.modular == m {
				return true
			}
		}
	}

	if p := oneHalfRootCaller(z); p != nil && p.config.Integer == z && p.modular == m && p.oneHalfRootExecuting() {
		return true
	}
	if c := algebraicNegCaller(z); c != nil && c.config.Rationals != nil && c.config.Rationals.Integers() == z && c.negation.phase >= 2 && c.negationExecuting() {
		p := c.config.Polynomials
		if p != nil && p.self == p && p.config.Integer == z && p.modular == m && p.algebraicNegCaller == c {
			return true
		}
	}
	for q := upolyDisplayCaller(z); q != nil; q = q.previousInteger {
		if q.actual() && q.owner.config.Integer == z && q.owner.modular == m && (q.executing || q.sourceExecuting || q.ownExecuting) {
			return true
		}
	}
	if p := knuthCaller(z); p != nil && p.config.Integer == z && p.modular == m && p.knuthExecuting() {
		return true
	}
	if p := rootUpperBoundCaller(z); p != nil && p.config.Integer == z && p.modular == m && p.rootUpperBoundExecuting() {
		return true
	}
	if p := removeOneHalfRootCaller(z); p != nil && p.config.Integer == z && p.modular == m && p.removeOneHalfRootExecuting() {
		return true
	}
	if p := integerEvaluationCaller(z); p != nil && p.config.Integer == z && p.modular == m && p.integerEvaluationExecuting() {
		return true
	}
	if p := rationalEvaluationCaller(z); p != nil && p.config.Integer == z && p.modular == m && p.rationalEvaluationExecuting() {
		return true
	}
	if p := binaryEvaluationCaller(z); p != nil && p.config.Integer == z && p.modular == m && p.binaryEvaluationExecuting() {
		return true
	}
	return false
}
