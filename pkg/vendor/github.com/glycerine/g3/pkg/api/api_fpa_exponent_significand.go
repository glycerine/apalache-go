package api

// Source: src/api/api_fpa.cpp:1208-1225.

import "github.com/glycerine/g3/pkg/ast"

// FloatingPointFromExponentSignificand constructs the source to_fp overload
// whose exponent is an Int expression and significand is a Real expression.
func (c *SMTContext) FloatingPointFromExponentSignificand(rounding, exponent, significand, target ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.requireFloatingPointConversionSource(rounding, APISortRoundingMode); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := c.requireFloatingPointConversionSource(exponent, APISortInt); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := c.requireFloatingPointConversionSource(significand, APISortReal); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.floatingPointToFormat(ast.FPAOpToFP, target, rounding, exponent, significand)
}
