package api

// Source: src/api/api_numeral.cpp:265-388.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

// NumeralSmall extracts a rational when numerator and denominator both fit
// signed 64-bit values.
func (c *SMTContext) NumeralSmall(value ast.ASTHandle) (numerator, denominator int64, matched bool, err error) {
	return c.NumeralRationalInt64(value)
}

func (c *SMTContext) NumeralInt32(value ast.ASTHandle) (int32, bool, error) {
	integer, matched, err := c.NumeralInt64(value)
	if err != nil || !matched || integer < math.MinInt32 || integer > math.MaxInt32 {
		return 0, false, err
	}
	return int32(integer), true, nil
}

func (c *SMTContext) NumeralUint32(value ast.ASTHandle) (uint32, bool, error) {
	integer, matched, err := c.NumeralUint64(value)
	if err != nil || !matched || integer > math.MaxUint32 {
		return 0, false, err
	}
	return uint32(integer), true, nil
}
