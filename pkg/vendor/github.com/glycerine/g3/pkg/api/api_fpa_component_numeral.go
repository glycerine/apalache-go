package api

// Source: src/api/api_fpa.cpp:341-379.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) FloatingPointNumeralComponents(sign bool, exponent int64, significand uint64, sort ast.ASTHandle) (ast.ASTHandle, error) {
	manager, _, err := c.floatingPointManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkFPANumeralComponents(sign, exponent, significand, sort)
}
