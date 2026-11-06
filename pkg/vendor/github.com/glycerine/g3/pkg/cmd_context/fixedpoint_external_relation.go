package cmd_context

// Source: src/api/api_datalog.cpp fixedpoint_context callback registration.

import (
	"github.com/glycerine/g3/pkg/ast"
	muzrel "github.com/glycerine/g3/pkg/muz/rel"
)

func (c *Context) SetFixedpointExternalRelationCallbacks(callbacks muzrel.ExternalRelationCallbacks) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.SetExternalRelationCallbacks(callbacks)
}

func (c *Context) NewFixedpointExternalRelation(sorts []ast.ASTHandle) (*muzrel.ExternalRelation, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	return engine.NewExternalRelation(sorts)
}
