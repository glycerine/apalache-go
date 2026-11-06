package api

// Source: src/api/api_fpa.cpp:908-1158.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) FloatingPointNumeralSign(value ast.ASTHandle) (int, error) {
	parts, err := c.floatingPointRawNumeralParts(value)
	if err != nil {
		return 0, err
	}
	if parts.Sign {
		return 1, nil
	}
	return 0, nil
}

func (c *SMTContext) FloatingPointNumeralSignificandUint64(value ast.ASTHandle) (uint64, error) {
	parts, err := c.floatingPointRawNumeralParts(value)
	if err != nil {
		return 0, err
	}
	n, ok := new(big.Int).SetString(parts.Significand, 10)
	if !ok || n.Sign() < 0 || !n.IsUint64() {
		return 0, ast.ErrASTDomain
	}
	return n.Uint64(), nil
}

func (c *SMTContext) FloatingPointNumeralExponentInt64(value ast.ASTHandle, biased bool) (int64, error) {
	parts, err := c.floatingPointRawNumeralParts(value)
	if err != nil {
		return 0, err
	}
	return floatingPointNumeralExponent(parts, biased)
}

// floatingPointNumeralExponent follows api_fpa.cpp's MPF field access, including
// its distinct zero, infinity and denormal cases.
func floatingPointNumeralExponent(parts ast.FPANumeralParts, biased bool) (int64, error) {
	if parts.Zero {
		return 0, nil
	}
	if parts.ExponentBits < 2 {
		return 0, ast.ErrASTDomain
	}
	top := new(big.Int).Lsh(big.NewInt(1), uint(parts.ExponentBits-1))
	var result *big.Int
	if parts.Infinite {
		result = top
	} else if parts.Denormal && !biased {
		result = new(big.Int).Sub(big.NewInt(2), top)
	} else {
		result = big.NewInt(parts.Exponent)
		if biased {
			result.Add(result, new(big.Int).Sub(top, big.NewInt(1)))
		}
	}
	if !result.IsInt64() {
		return 0, ast.ErrASTDomain
	}
	return result.Int64(), nil
}
