package realclosure

import (
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
)

// invertAlgebraicPolynomial follows inv_algebraic's Q/R recurrence. The
// current retained algebraic extensions have rational defining coefficients.
func invertAlgebraicPolynomial(q, p []big.Rat) ([]big.Rat, bool) {
	if len(q) == 0 || len(q) >= len(p) {
		return nil, false
	}
	Q := bigPolynomialCopy(q)
	R := make([]big.Rat, 1)
	R[0].SetInt64(1)
	for {
		if len(Q) == 1 {
			var inverse big.Rat
			inverse.Inv(&Q[0])
			return bigPolynomialScale(R, &inverse), true
		}
		quotient, remainder, ok := bigPolynomialDivRem(p, Q)
		if !ok || len(remainder) == 0 {
			return nil, false
		}
		Q = bigPolynomialNeg(remainder)
		aux := bigPolynomialMul(R, quotient)
		R, ok = bigPolynomialMod(aux, p)
		if !ok {
			return nil, false
		}
	}
}

func (s *realValueStore) inverseNormalizedValue(a *realValue) (*realValue, error) {
	if !s.realOperand(a) || a == nil {
		return nil, arena.ErrInvalidSlot
	}
	if a.isRational {
		one, err := s.newRational(1, 1)
		if err != nil {
			return nil, err
		}
		out, err := s.divideRationals(one, a)
		releaseErr := s.release(one)
		if err != nil {
			return nil, err
		}
		if releaseErr != nil {
			_ = s.release(out)
			return nil, releaseErr
		}
		return out, nil
	}
	q, err := s.rationalPolynomialValues(a.functionNumerator.coefficients)
	if err != nil {
		return nil, err
	}
	p, err := s.rationalPolynomialValues(a.root.polynomial.coefficients)
	if err != nil {
		return nil, err
	}
	q, ok := bigPolynomialMod(q, p)
	if !ok || len(q) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	h, ok := invertAlgebraicPolynomial(q, p)
	if !ok {
		return nil, arena.ErrInvalidSlot
	}
	values, err := s.ownBigPolynomial(h)
	if err != nil {
		return nil, err
	}
	approximation, err := s.newApproximation(a.root)
	if err != nil {
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	if err = approximation.Inv(a.functionInterval, 32); err != nil {
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
