package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (c *arithmeticCompleteProject) apply(formula ast.ASTRef, definition *ast.ASTRef, branch uint32) (result error) {
	service := c.plugin.manager.NumeralService()
	value, e := service.NewValue()
	if e != nil {
		return e
	}
	defer func() { result = errors.Join(result, service.Del(&value)) }()
	if e = service.SetUnsigned(&value, branch); e != nil {
		return e
	}
	return c.plugin.AssignSubstituteComplete(c.variable, formula, &value, definition)
}
