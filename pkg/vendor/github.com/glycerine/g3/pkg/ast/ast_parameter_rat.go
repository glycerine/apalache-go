package ast

import (
	"errors"
	"math/big"
)

// NewParameterRat copies a Go rational into an owning AST parameter, using
// the same numeral service and lifetime as NewParameterRational.
func (m *ASTManager) NewParameterRat(value *big.Rat) (result ASTParameter, err error) {
	if value == nil {
		return ASTParameter{}, ErrASTDomain
	}
	n, err := m.numerals.NewValue()
	if err != nil {
		return ASTParameter{}, err
	}
	defer func() { err = errors.Join(err, m.numerals.CleanupValue(&n)) }()
	if err = m.numerals.SetString(&n, value.RatString()); err != nil {
		return ASTParameter{}, err
	}
	return m.NewParameterRational(&n)
}
