package api

// Source: src/api/api_fpa.cpp:182-184,190-192,198-200,206-208.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) FloatingPointSort16() (ast.ASTHandle, error) {
	return c.FloatingPointHalfSort()
}

func (c *SMTContext) FloatingPointSort32() (ast.ASTHandle, error) {
	return c.FloatingPointSingleSort()
}

func (c *SMTContext) FloatingPointSort64() (ast.ASTHandle, error) {
	return c.FloatingPointDoubleSort()
}

func (c *SMTContext) FloatingPointSort128() (ast.ASTHandle, error) {
	return c.FloatingPointQuadrupleSort()
}
