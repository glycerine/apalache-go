package realclosure

import "github.com/glycerine/g3/pkg/arena"

// multiplyValues covers rational products and the retained algebraic n/1
// scaling path. Algebraic-by-algebraic reduction needs normalize_algebraic
// and is deliberately not represented by this concrete packet.
func (s *realValueStore) multiplyValues(a, b *realValue) (*realValue, error) {
	if !s.realOperand(a) || !s.realOperand(b) {
		return nil, arena.ErrInvalidSlot
	}
	if a == nil || b == nil {
		return nil, nil
	}
	if a.isRational && b.isRational {
		return s.multiplyRationals(a, b)
	}
	if !a.isRational && !b.isRational {
		return nil, arena.ErrInvalidSlot
	}
	algebraic, scalar := a, b
	if algebraic.isRational {
		algebraic, scalar = b, a
	}
	values := make([]*realValue, len(algebraic.functionNumerator.coefficients))
	for i, coefficient := range algebraic.functionNumerator.coefficients {
		value, err := s.multiplyRationals(coefficient, scalar)
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
	scalarInterval, err := s.approximation(scalar, algebraic.root)
	if err != nil {
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	approximation, err := s.newApproximation(algebraic.root)
	if err != nil {
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	if err = approximation.Mul(algebraic.functionInterval, scalarInterval); err != nil {
		_ = approximation.Close()
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	out, err := s.publishFunctionValue(algebraic.root, values, approximation)
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
