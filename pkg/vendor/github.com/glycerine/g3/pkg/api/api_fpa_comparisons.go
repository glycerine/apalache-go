package api

// Source: src/api/api_fpa.cpp:552-625.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) FloatingPointLE(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpLe, left, right)
}
func (c *SMTContext) FloatingPointLT(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpLt, left, right)
}
func (c *SMTContext) FloatingPointGE(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpGe, left, right)
}
func (c *SMTContext) FloatingPointGT(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpGt, left, right)
}
func (c *SMTContext) FloatingPointEqual(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpEq, left, right)
}
