package api

// Source: src/api/api_fpa.cpp:1003-1036.

import (
	"math/big"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

// FloatingPointNumeralSignificandString returns the source normalized decimal
// significand. Dyadic values terminate, so source decimal rendering has no
// repeating-value marker.
func (c *SMTContext) FloatingPointNumeralSignificandString(value ast.ASTHandle) (string, error) {
	parts, err := c.floatingPointRawNumeralParts(value)
	if err != nil {
		return "", err
	}
	if parts.Infinite {
		return "0", nil
	}
	numerator, ok := new(big.Int).SetString(parts.Significand, 10)
	if !ok || numerator.Sign() < 0 || parts.SignificandBits == 0 {
		return "", ast.ErrASTDomain
	}
	denominator := new(big.Int).Lsh(big.NewInt(1), uint(parts.SignificandBits-1))
	if !parts.Denormal {
		numerator.Add(numerator, denominator)
	}
	text := new(big.Rat).SetFrac(numerator, denominator).FloatString(int(parts.SignificandBits))
	text = strings.TrimRight(text, "0")
	text = strings.TrimSuffix(text, ".")
	if text == "" {
		text = "0"
	}
	return text, nil
}
