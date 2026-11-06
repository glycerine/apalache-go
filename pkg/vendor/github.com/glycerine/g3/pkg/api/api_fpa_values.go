package api

// Source: src/api/api_fpa.cpp:210-273.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) FloatingPointNaN(sort ast.ASTHandle) (ast.ASTHandle, error) {
	m, _, err := c.floatingPointManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// Match is_fp_sort before entering the asserting fpa_util accessor.
	_, _, matched, err := m.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !matched {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return m.MkFPANaNOfSort(sort)
}

func (c *SMTContext) FloatingPointInfinity(sort ast.ASTHandle, negative bool) (ast.ASTHandle, error) {
	m, _, err := c.floatingPointManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// Match is_fp_sort before entering the asserting fpa_util accessor.
	_, _, matched, err := m.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !matched {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return m.MkFPAInfinityOfSort(sort, negative)
}

func (c *SMTContext) FloatingPointZero(sort ast.ASTHandle, negative bool) (ast.ASTHandle, error) {
	m, _, err := c.floatingPointManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// Match is_fp_sort before entering the asserting fpa_util accessor.
	_, _, matched, err := m.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !matched {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return m.MkFPAZeroOfSort(sort, negative)
}

func (c *SMTContext) FloatingPointFromComponents(sign, exponent, significand ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpFP, sign, exponent, significand)
}
