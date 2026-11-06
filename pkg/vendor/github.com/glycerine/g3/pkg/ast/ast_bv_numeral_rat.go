package ast

import (
	"errors"
	"fmt"
	"math/big"
)

// BitVectorNumeralRat creates a bit-vector numeral from a nonnegative integer.
func (m *ASTManager) BitVectorNumeralRat(value *big.Rat, bits uint32) (result ASTHandle, err error) {
	if value == nil || !value.IsInt() {
		return ASTHandle{}, fmt.Errorf("g3: invalid bit-vector numeral")
	}
	n, err := m.numerals.NewValue()
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, m.numerals.CleanupValue(&n)) }()
	if err = m.numerals.SetString(&n, value.RatString()); err != nil {
		return ASTHandle{}, err
	}
	return m.BitVectorNumeral(&n, bits)
}
