package api

// Source: src/api/api_fpa.cpp:284-339.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) FloatingPointNumeralFloat32(value float32, sort ast.ASTHandle) (ast.ASTHandle, error) {
	manager, _, err := c.floatingPointManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkFPANumeralFloat32(value, sort)
}

func (c *SMTContext) FloatingPointNumeralFloat64(value float64, sort ast.ASTHandle) (ast.ASTHandle, error) {
	manager, _, err := c.floatingPointManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkFPANumeralFloat64(value, sort)
}

func (c *SMTContext) FloatingPointNumeralInt32(value int32, sort ast.ASTHandle) (ast.ASTHandle, error) {
	manager, _, err := c.floatingPointManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkFPANumeralInt32(value, sort)
}
