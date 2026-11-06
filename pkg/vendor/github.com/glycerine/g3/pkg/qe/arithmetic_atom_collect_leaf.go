package qe

import "github.com/glycerine/g3/pkg/ast"

func (c *arithmeticAtomCollector) leaf(expression ast.ASTHandle, positive bool) error {
	if positive {
		return c.result.AddPositive(expression)
	}
	return c.result.AddNegative(expression)
}
