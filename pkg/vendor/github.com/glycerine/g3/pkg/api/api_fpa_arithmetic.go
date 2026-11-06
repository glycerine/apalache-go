package api

// Source: src/api/api_fpa.cpp:372-550.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) FloatingPointAbs(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpAbs, value)
}
func (c *SMTContext) FloatingPointNeg(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpNeg, value)
}
func (c *SMTContext) FloatingPointAdd(rounding, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpAdd, rounding, left, right)
}
func (c *SMTContext) FloatingPointSub(rounding, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpSub, rounding, left, right)
}
func (c *SMTContext) FloatingPointMul(rounding, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpMul, rounding, left, right)
}
func (c *SMTContext) FloatingPointDiv(rounding, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpDiv, rounding, left, right)
}
func (c *SMTContext) FloatingPointFusedMultiplyAdd(rounding, first, second, third ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpFMA, rounding, first, second, third)
}
func (c *SMTContext) FloatingPointSqrt(rounding, value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpSqrt, rounding, value)
}
func (c *SMTContext) FloatingPointRemainder(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpRem, left, right)
}
func (c *SMTContext) FloatingPointRoundToIntegral(rounding, value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpRoundToIntegral, rounding, value)
}
func (c *SMTContext) FloatingPointMin(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpMin, left, right)
}
func (c *SMTContext) FloatingPointMax(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpMax, left, right)
}
