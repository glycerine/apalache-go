package api

// Source: src/api/api_fpa.cpp:42-51,164-208,871-897.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) FloatingPointRoundingModeSort() (ast.ASTHandle, error) {
	m, _, err := c.floatingPointManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.RoundingModeSort()
}

func (c *SMTContext) FloatingPointSort(exponentBits, significandBits uint32) (ast.ASTHandle, error) {
	if exponentBits < 2 || significandBits < 3 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	m, _, err := c.floatingPointManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.FloatingPointSort(exponentBits, significandBits)
}

func (c *SMTContext) FloatingPointHalfSort() (ast.ASTHandle, error) {
	return c.FloatingPointSort(5, 11)
}
func (c *SMTContext) FloatingPointSingleSort() (ast.ASTHandle, error) {
	return c.FloatingPointSort(8, 24)
}
func (c *SMTContext) FloatingPointDoubleSort() (ast.ASTHandle, error) {
	return c.FloatingPointSort(11, 53)
}
func (c *SMTContext) FloatingPointQuadrupleSort() (ast.ASTHandle, error) {
	return c.FloatingPointSort(15, 113)
}

func (c *SMTContext) FloatingPointSortShape(sort ast.ASTHandle) (exponentBits, significandBits uint32, err error) {
	m, _, err := c.floatingPointManager()
	if err != nil {
		return 0, 0, err
	}
	exponentBits, significandBits, matched, err := m.FloatingPointSortShape(sort)
	if err != nil {
		return 0, 0, err
	}
	if !matched {
		return 0, 0, ast.ErrASTDomain
	}
	return exponentBits, significandBits, nil
}
