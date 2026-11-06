package api

// Source: src/api/api_fpa.cpp:64-72,86-94,108-116,130-138,152-160.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) FloatingPointRNE() (ast.ASTHandle, error) {
	return c.FloatingPointRoundNearestEven()
}

func (c *SMTContext) FloatingPointRNA() (ast.ASTHandle, error) {
	return c.FloatingPointRoundNearestAway()
}

func (c *SMTContext) FloatingPointRTP() (ast.ASTHandle, error) {
	return c.FloatingPointRoundTowardPositiveValue()
}

func (c *SMTContext) FloatingPointRTN() (ast.ASTHandle, error) {
	return c.FloatingPointRoundTowardNegativeValue()
}

func (c *SMTContext) FloatingPointRTZ() (ast.ASTHandle, error) {
	return c.FloatingPointRoundTowardZeroValue()
}
