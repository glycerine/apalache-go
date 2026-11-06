package api

// Source: src/api/api_fpa.cpp:1230-1323.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) FloatingPointNumeralClass(value ast.ASTHandle, kind ast.FPAClassificationKind) (bool, error) {
	m, _, err := c.floatingPointManager()
	if err != nil {
		return false, err
	}
	found, classified, err := m.ClassifyFPANumeral(value, kind)
	if err != nil {
		return false, err
	}
	if !found {
		return false, ast.ErrASTDomain
	}
	return classified, nil
}

func (c *SMTContext) FloatingPointNumeralIsNaN(value ast.ASTHandle) (bool, error) {
	return c.FloatingPointNumeralClass(value, ast.FPAIsNaN)
}
func (c *SMTContext) FloatingPointNumeralIsInfinite(value ast.ASTHandle) (bool, error) {
	return c.FloatingPointNumeralClass(value, ast.FPAIsInfinite)
}
func (c *SMTContext) FloatingPointNumeralIsZero(value ast.ASTHandle) (bool, error) {
	return c.FloatingPointNumeralClass(value, ast.FPAIsZero)
}
func (c *SMTContext) FloatingPointNumeralIsNormal(value ast.ASTHandle) (bool, error) {
	return c.FloatingPointNumeralClass(value, ast.FPAIsNormal)
}
func (c *SMTContext) FloatingPointNumeralIsSubnormal(value ast.ASTHandle) (bool, error) {
	return c.FloatingPointNumeralClass(value, ast.FPAIsSubnormal)
}
func (c *SMTContext) FloatingPointNumeralIsPositive(value ast.ASTHandle) (bool, error) {
	return c.FloatingPointNumeralClass(value, ast.FPAIsPositive)
}
func (c *SMTContext) FloatingPointNumeralIsNegative(value ast.ASTHandle) (bool, error) {
	return c.FloatingPointNumeralClass(value, ast.FPAIsNegative)
}
