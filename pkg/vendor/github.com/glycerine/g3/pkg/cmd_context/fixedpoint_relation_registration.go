package cmd_context

// Source: src/api/api_datalog.cpp:431-436.

import "github.com/glycerine/g3/pkg/ast"

// RegisterFixedpointRelation registers an existing Boolean function
// declaration with this command view's independent fixedpoint engine.
func (c *Context) RegisterFixedpointRelation(declaration ast.ASTHandle) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.RegisterPredicate(declaration)
}
