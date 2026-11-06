package realclosure

import "github.com/glycerine/g3/pkg/arena"

// addValues retains the source rank-equal algebraic n/1 path and the
// algebraic-plus-rational path used by real-closure polynomial arithmetic.
func (s *realValueStore) addValues(a, b *realValue) (*realValue, error) {
	if !s.realOperand(a) || !s.realOperand(b) {
		return nil, arena.ErrInvalidSlot
	}
	if a == nil {
		if err := s.retain(b); err != nil {
			return nil, err
		}
		return b, nil
	}
	if b == nil {
		if err := s.retain(a); err != nil {
			return nil, err
		}
		return a, nil
	}
	if a.isRational && b.isRational {
		return s.addRationals(a, b)
	}
	root := a.root
	if a.isRational {
		root = b.root
	}
	if !a.isRational && !b.isRational && !sameAlgebraicRoot(a, b) {
		return nil, arena.ErrInvalidSlot
	}
	left := []*realValue{a}
	if !a.isRational {
		left = a.functionNumerator.coefficients
	}
	right := []*realValue{b}
	if !b.isRational {
		right = b.functionNumerator.coefficients
	}
	n := len(left)
	if len(right) > n {
		n = len(right)
	}
	values := make([]*realValue, n)
	for i := 0; i < n; i++ {
		var x, y *realValue
		if i < len(left) {
			x = left[i]
		}
		if i < len(right) {
			y = right[i]
		}
		value, err := s.addRationals(x, y)
		if err != nil {
			_ = s.releaseTemporaryCoefficients(values)
			return nil, err
		}
		values[i] = value
	}
	values = trimRealCoefficients(values)
	if len(values) == 0 {
		return nil, nil
	}
	leftInterval, err := s.approximation(a, root)
	if err != nil {
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	rightInterval, err := s.approximation(b, root)
	if err != nil {
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	approximation, err := s.newApproximation(root)
	if err != nil {
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	if err = approximation.Add(leftInterval, rightInterval); err != nil {
		_ = approximation.Close()
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	out, err := s.publishFunctionValue(root, values, approximation)
	closeErr := approximation.Close()
	releaseErr := s.releaseTemporaryCoefficients(values)
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		_ = s.release(out)
		return nil, closeErr
	}
	if releaseErr != nil {
		_ = s.release(out)
		return nil, releaseErr
	}
	return out, nil
}
