package polynomial

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// DefiningPolynomialText returns the source-order integer coefficients used by
// arithmetic root-object purification. It is the concrete retained consumer of
// AlgebraicRootCore.GetPolynomial; the returned strings own no numeral state.
func (m *AlgebraicRootCore) DefiningPolynomialText(a *Algebraic) (result []string, err error) {
	if m == nil || m.config.Polynomials == nil {
		return nil, ErrAlgebraicOwner
	}
	p, err := m.config.Polynomials.NewPolynomial()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, m.config.Polynomials.DelPolynomial(&p)) }()
	if err = m.GetPolynomial(a, p); err != nil {
		return nil, err
	}
	v, err := m.config.Polynomials.wholeView(p)
	if err != nil {
		return nil, err
	}
	result = make([]string, 0, v.count)
	for j := uint32(0); j < v.count; j++ {
		coefficient, e := m.config.Polynomials.read(v, j)
		if e != nil {
			return nil, e
		}
		text, e := m.config.Rationals.Integers().String(coefficient)
		if e != nil {
			return nil, e
		}
		result = append(result, text)
	}
	return result, nil
}

// IrrationalBoundsText returns copies of the stored strict isolating bounds in
// ordinary rational syntax for the retained purifier.
func (m *AlgebraicRootCore) IrrationalBoundsText(a *Algebraic) (lower, upper string, err error) {
	if m == nil || m.config.Rationals == nil {
		return "", "", ErrAlgebraicOwner
	}
	q := m.config.Rationals
	lo, hi := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&hi), q.Del(&lo)) }()
	if err = m.GetLowerRational(a, &lo); err != nil {
		return "", "", err
	}
	if err = m.GetUpperRational(a, &hi); err != nil {
		return "", "", err
	}
	lower, err = q.String(&lo)
	if err != nil {
		return "", "", err
	}
	upper, err = q.String(&hi)
	return lower, upper, err
}
