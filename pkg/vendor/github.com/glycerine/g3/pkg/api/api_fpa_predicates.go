package api

// Source: src/api/api_fpa.cpp:627-731.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) FloatingPointIsNormal(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(int32(ast.FPAIsNormal), value)
}
func (c *SMTContext) FloatingPointIsSubnormal(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(int32(ast.FPAIsSubnormal), value)
}
func (c *SMTContext) FloatingPointIsZero(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(int32(ast.FPAIsZero), value)
}
func (c *SMTContext) FloatingPointIsInfinite(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(int32(ast.FPAIsInfinite), value)
}
func (c *SMTContext) FloatingPointIsNaN(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(int32(ast.FPAIsNaN), value)
}
func (c *SMTContext) FloatingPointIsNegative(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(int32(ast.FPAIsNegative), value)
}
func (c *SMTContext) FloatingPointIsPositive(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(int32(ast.FPAIsPositive), value)
}
