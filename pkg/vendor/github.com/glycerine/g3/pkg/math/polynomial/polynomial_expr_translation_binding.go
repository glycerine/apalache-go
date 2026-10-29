package polynomial

// Source binding: src/math/polynomial/polynomial.h:561-577,670-676.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// IntegerManager returns the borrowed coefficient manager used by this core.
// It supports concrete retained translators without exposing implementation
// storage or introducing a second numeral owner.
func (m *MultivariatePolynomialCore) IntegerManager() (*unumeral.IntegerManager, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.integer, nil
}

// MonomialManager returns the borrowed monomial manager used by this core.
func (m *MultivariatePolynomialCore) MonomialManager() (*MonomialManager, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.monomials, nil
}

// NumVars is the source manager::num_vars forwarding operation.
func (m *MultivariatePolynomialCore) NumVars() (uint32, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	return m.monomials.NumVars()
}

// MkVar is the source manager::mk_var forwarding operation.
func (m *MultivariatePolynomialCore) MkVar() (uint32, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	return m.monomials.MkVar()
}

// Zero returns the borrowed canonical zero polynomial.
func (m *MultivariatePolynomialCore) Zero() (Polynomial, error) {
	if err := m.check(); err != nil {
		return Polynomial{}, err
	}
	return m.zero, nil
}
