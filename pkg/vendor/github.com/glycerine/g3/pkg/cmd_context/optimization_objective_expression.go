package cmd_context

// Source: src/opt/opt_context.cpp:194-219, non-MaxSMT objectives.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (c *Context) optimizationTermExpression(objective *optimizationObjective) (ast.ASTRef, error) {
	if objective == nil || objective.soft {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	handle := objective.expression
	var err error
	if objective.maximize {
		if objective.bitVector {
			handle, err = c.manager.MkBitVectorNeg(handle)
		} else {
			handle, err = c.manager.MkArithmeticNeg(handle)
		}
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	return c.manager.NewRef(handle)
}
