package ast

import (
	"errors"
	"fmt"
	"math/big"
)

// ArithmeticNumeralRat creates an exact arithmetic numeral from an idiomatic
// Go rational while keeping the numeral service's source ownership internal.
func (m *ASTManager) ArithmeticNumeralRat(value *big.Rat, integer bool) (result ASTHandle, err error) {
	if value == nil {
		return ASTHandle{}, fmt.Errorf("g3: nil arithmetic numeral")
	}
	n, err := m.numerals.NewValue()
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, m.numerals.CleanupValue(&n)) }()
	if err = m.numerals.SetString(&n, value.RatString()); err != nil {
		return ASTHandle{}, err
	}
	return m.ArithmeticNumeral(&n, integer)
}
