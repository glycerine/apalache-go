package api

// Source: api_ast.cpp simplify's cancel_eh and context::set_interruptable
// scope at lines 679-696.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *SMTContext) simplifyWithAPIInterrupt(expression ast.ASTHandle, parameters *uparams.ParamsRef) (result ast.ASTRef, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return result, err
	}
	handler := newSMTContextCancelHandler(manager.ResourceLimit())
	scope, err := newSMTContextInterruptScope(c, handler)
	if err != nil {
		handler.close()
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		handler.close()
	}()
	return c.command.Simplify(expression, parameters)
}
