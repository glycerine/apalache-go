package realclosure

import "github.com/glycerine/g3/pkg/arena"

// multiplyAlgebraicValues is source mul_p_p plus normalize_algebraic for two
// denominator-one values over the same concrete algebraic extension.
func (s *realValueStore) multiplyAlgebraicValues(a, b *realValue) (*realValue, error) {
	if !s.realOperand(a) || !s.realOperand(b) || !sameAlgebraicRoot(a, b) {
		return nil, arena.ErrInvalidSlot
	}
	product, err := s.multiplyRationalPolynomials(a.functionNumerator.coefficients, b.functionNumerator.coefficients)
	if err != nil {
		return nil, err
	}
	reduced, err := s.reduceAlgebraicNumerator(a.root, product)
	releaseProduct := s.releaseTemporaryCoefficients(product)
	if err != nil {
		return nil, err
	}
	if releaseProduct != nil {
		_ = s.releaseTemporaryCoefficients(reduced)
		return nil, releaseProduct
	}
	if len(reduced) == 0 {
		return nil, nil
	}
	approximation, err := s.newApproximation(a.root)
	if err != nil {
		_ = s.releaseTemporaryCoefficients(reduced)
		return nil, err
	}
	if err = approximation.Mul(a.functionInterval, b.functionInterval); err != nil {
		_ = approximation.Close()
		_ = s.releaseTemporaryCoefficients(reduced)
		return nil, err
	}
	out, err := s.publishFunctionValue(a.root, reduced, approximation)
	closeErr := approximation.Close()
	releaseErr := s.releaseTemporaryCoefficients(reduced)
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
