package api

// Source: src/api/api_fpa.cpp:871-897.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) FloatingPointExponentBits(sort ast.ASTHandle) (uint32, error) {
	exponentBits, _, err := c.FloatingPointSortShape(sort)
	return exponentBits, err
}

func (c *SMTContext) FloatingPointSignificandBits(sort ast.ASTHandle) (uint32, error) {
	_, significandBits, err := c.FloatingPointSortShape(sort)
	return significandBits, err
}
