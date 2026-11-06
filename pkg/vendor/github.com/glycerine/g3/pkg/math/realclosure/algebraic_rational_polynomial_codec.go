package realclosure

import (
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) rationalPolynomialValues(values []*realValue) ([]big.Rat, error) {
	out := make([]big.Rat, len(values))
	for i, value := range values {
		if !s.rationalOperand(value) {
			return nil, arena.ErrInvalidSlot
		}
		if value == nil {
			continue
		}
		text, err := s.numbers.String(&value.rational)
		if err != nil {
			return nil, err
		}
		if _, ok := out[i].SetString(text); !ok {
			return nil, arena.ErrInvalidSlot
		}
	}
	return trimBigPolynomial(out), nil
}

func (s *realValueStore) ownBigPolynomial(values []big.Rat) ([]*realValue, error) {
	out := make([]*realValue, len(values))
	for i := range values {
		if values[i].Sign() == 0 {
			continue
		}
		q := numeral.NewRational()
		if err := s.numbers.SetString(&q, values[i].RatString()); err != nil {
			_ = s.releaseTemporaryCoefficients(out)
			return nil, err
		}
		value, err := s.ownNonzeroRational(&q)
		if err != nil {
			_ = s.releaseTemporaryCoefficients(out)
			return nil, err
		}
		out[i] = value
	}
	return trimRealCoefficients(out), nil
}

func trimBigPolynomial(values []big.Rat) []big.Rat {
	for len(values) > 0 && values[len(values)-1].Sign() == 0 {
		values = values[:len(values)-1]
	}
	return values
}
