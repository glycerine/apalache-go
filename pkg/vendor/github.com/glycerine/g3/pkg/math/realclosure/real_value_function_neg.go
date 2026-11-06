package realclosure

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) negateValue(a *realValue) (*realValue, error) {
	if !s.realOperand(a) {
		return nil, arena.ErrInvalidSlot
	}
	if a == nil || a.isRational {
		return s.negateRational(a)
	}
	values := make([]*realValue, len(a.functionNumerator.coefficients))
	for i, coefficient := range a.functionNumerator.coefficients {
		value, err := s.negateRational(coefficient)
		if err != nil {
			_ = s.releaseTemporaryCoefficients(values)
			return nil, err
		}
		values[i] = value
	}
	approximation, err := s.newApproximation(a.root)
	if err != nil {
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	if err = approximation.Neg(a.functionInterval); err != nil {
		_ = approximation.Close()
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	out, err := s.publishFunctionValue(a.root, values, approximation)
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

func (s *realValueStore) subtractValues(a, b *realValue) (*realValue, error) {
	negative, err := s.negateValue(b)
	if err != nil {
		return nil, err
	}
	out, err := s.addValues(a, negative)
	releaseErr := s.release(negative)
	if err != nil {
		return nil, err
	}
	if releaseErr != nil {
		_ = s.release(out)
		return nil, releaseErr
	}
	return out, nil
}
