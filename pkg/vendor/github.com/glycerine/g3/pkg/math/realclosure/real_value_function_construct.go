package realclosure

import "github.com/glycerine/g3/pkg/arena"

// publishFunctionValue copies a normalized numerator into a value that owns
// the algebraic extension and its independently owned approximation interval.
func (s *realValueStore) publishFunctionValue(root *algebraicRoot, coefficients []*realValue, approximation *BinaryInterval) (*realValue, error) {
	coefficients = trimRealCoefficients(coefficients)
	if len(coefficients) == 0 {
		return nil, nil
	}
	if root == nil || root.deleted || root.owner == nil || root.owner.values != s || approximation == nil {
		return nil, arena.ErrInvalidSlot
	}
	if len(coefficients) == 1 {
		if err := s.retain(coefficients[0]); err != nil {
			return nil, err
		}
		return coefficients[0], nil
	}
	if err := root.retain(); err != nil {
		return nil, err
	}
	v := &realValue{store: s, root: root, refs: 1}
	if err := s.setPolynomial(&v.functionNumerator, coefficients); err != nil {
		_ = root.release()
		return nil, err
	}
	interval, err := NewBinaryInterval(root.owner.intervals)
	if err != nil {
		_ = s.resetPolynomial(&v.functionNumerator)
		_ = root.release()
		return nil, err
	}
	if err = interval.SetFrom(approximation); err != nil {
		_ = interval.Close()
		_ = s.resetPolynomial(&v.functionNumerator)
		_ = root.release()
		return nil, err
	}
	v.functionInterval = interval
	return v, nil
}

func (s *realValueStore) releaseTemporaryCoefficients(values []*realValue) error {
	for i := range values {
		if err := s.release(values[i]); err != nil {
			return err
		}
		values[i] = nil
	}
	return nil
}
