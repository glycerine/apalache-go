package polynomial

// Source: src/math/polynomial/polynomial.cpp:2503-2638.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// GCDSimplifyRelation selects the source integer-content treatment. Equality
// divides every coefficient; strict relations preserve their integer solution
// set by rounding the constant term.
type GCDSimplifyRelation uint8

const (
	GCDSimplifyEquality GCDSimplifyRelation = iota
	GCDSimplifyLess
	GCDSimplifyGreater
)

// GCDSimplifyInto removes the greatest common divisor of the retained
// coefficients and publishes an independently owned result in out.
func (m *MultivariatePolynomialCore) GCDSimplifyInto(p Polynomial, relation GCDSimplifyRelation, out *PolynomialRef) error {
	if out == nil || out.owner != m {
		return arena.ErrConfig
	}
	if relation > GCDSimplifyGreater {
		return arena.ErrConfig
	}
	zero, err := m.IsZero(p)
	if err != nil || zero {
		if err != nil {
			return err
		}
		return out.Assign(p)
	}
	g, coefficients, monomials, err := m.polynomialContent(p, relation)
	if err != nil {
		return err
	}
	if g.Sign() == 0 || g.Cmp(big.NewInt(1)) == 0 {
		return out.Assign(p)
	}
	for i := range coefficients {
		constant, cause := m.gcdSimplifyConstant(monomials[i])
		if cause != nil {
			return cause
		}
		if constant && relation != GCDSimplifyEquality {
			if relation == GCDSimplifyGreater {
				coefficients[i] = integerCeilQuotient(coefficients[i], g)
			} else {
				coefficients[i] = integerFloorQuotient(coefficients[i], g)
			}
		} else {
			coefficients[i] = new(big.Int).Quo(coefficients[i], g)
		}
	}
	return m.publishGCDSimplification(coefficients, monomials, out)
}

func integerFloorQuotient(a, b *big.Int) *big.Int {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(a, b, r)
	if r.Sign() < 0 {
		q.Sub(q, big.NewInt(1))
	}
	return q
}

func integerCeilQuotient(a, b *big.Int) *big.Int {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(a, b, r)
	if r.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	return q
}

func (m *MultivariatePolynomialCore) publishGCDSimplification(values []*big.Int, monomials []Monomial, out *PolynomialRef) (err error) {
	z, err := m.IntegerManager()
	if err != nil {
		return err
	}
	coefficients := make([]unumeral.Int, len(values))
	defer func() {
		for i := range coefficients {
			err = errors.Join(err, z.Del(&coefficients[i]))
		}
	}()
	for i := range values {
		if err = z.SetString(&coefficients[i], values[i].String()); err != nil {
			return err
		}
	}
	result, err := m.MkTerms(coefficients, monomials)
	if err != nil {
		return err
	}
	return out.Assign(result)
}
