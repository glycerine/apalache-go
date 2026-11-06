package realclosure

// Source: src/math/realclosure/realclosure.cpp:4917-5050,5052-5129,
// 5148-5287. This is the concrete retained rational/algebraic subset.

func (s *realValueStore) realOperand(v *realValue) bool {
	if v == nil {
		return true
	}
	if s == nil || v.store != s || v.deleted || v.refs == 0 {
		return false
	}
	if v.isRational {
		return true
	}
	return v.root != nil && !v.root.deleted && v.functionInterval != nil && len(v.functionNumerator.coefficients) > 0
}

func sameAlgebraicRoot(a, b *realValue) bool {
	return a != nil && b != nil && !a.isRational && !b.isRational && a.root == b.root
}

func trimRealCoefficients(values []*realValue) []*realValue {
	for len(values) > 0 && values[len(values)-1] == nil {
		values = values[:len(values)-1]
	}
	return values
}
