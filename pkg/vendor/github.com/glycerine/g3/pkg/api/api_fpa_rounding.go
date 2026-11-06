package api

// Source: src/api/api_fpa.cpp:53-160.

import (
	"github.com/glycerine/g3/pkg/ast"
)

type FloatingPointRoundingMode uint8

const (
	FloatingPointRoundNearestTiesToEven FloatingPointRoundingMode = iota
	FloatingPointRoundNearestTiesToAway
	FloatingPointRoundTowardPositive
	FloatingPointRoundTowardNegative
	FloatingPointRoundTowardZero
)

func (c *SMTContext) FloatingPointRounding(mode FloatingPointRoundingMode) (ast.ASTHandle, error) {
	var kind int32
	switch mode {
	case FloatingPointRoundNearestTiesToEven:
		kind = ast.FPAOpRMNearestEven
	case FloatingPointRoundNearestTiesToAway:
		kind = ast.FPAOpRMNearestAway
	case FloatingPointRoundTowardPositive:
		kind = ast.FPAOpRMTowardPositive
	case FloatingPointRoundTowardNegative:
		kind = ast.FPAOpRMTowardNegative
	case FloatingPointRoundTowardZero:
		kind = ast.FPAOpRMTowardZero
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return c.floatingPointApp(kind)
}

func (c *SMTContext) FloatingPointRoundNearestEven() (ast.ASTHandle, error) {
	return c.FloatingPointRounding(FloatingPointRoundNearestTiesToEven)
}
func (c *SMTContext) FloatingPointRoundNearestAway() (ast.ASTHandle, error) {
	return c.FloatingPointRounding(FloatingPointRoundNearestTiesToAway)
}
func (c *SMTContext) FloatingPointRoundTowardPositiveValue() (ast.ASTHandle, error) {
	return c.FloatingPointRounding(FloatingPointRoundTowardPositive)
}
func (c *SMTContext) FloatingPointRoundTowardNegativeValue() (ast.ASTHandle, error) {
	return c.FloatingPointRounding(FloatingPointRoundTowardNegative)
}
func (c *SMTContext) FloatingPointRoundTowardZeroValue() (ast.ASTHandle, error) {
	return c.FloatingPointRounding(FloatingPointRoundTowardZero)
}
