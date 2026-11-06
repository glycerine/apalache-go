package api

// Source: src/api/api_fpa.cpp:1075-1114.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) floatingPointRawNumeralParts(value ast.ASTHandle) (ast.FPANumeralParts, error) {
	manager, _, err := c.floatingPointManager()
	if err != nil {
		return ast.FPANumeralParts{}, err
	}
	parts, matched, err := manager.ReadFPANumeralParts(value)
	if err != nil {
		return ast.FPANumeralParts{}, err
	}
	if !matched {
		return ast.FPANumeralParts{}, ast.ErrASTDomain
	}
	return parts, nil
}

func (c *SMTContext) FloatingPointNumeralExponentString(value ast.ASTHandle, biased bool) (string, error) {
	exponent, err := c.FloatingPointNumeralExponentInt64(value, biased)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(exponent, 10), nil
}
