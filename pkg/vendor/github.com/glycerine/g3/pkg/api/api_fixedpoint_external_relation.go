package api

// Source: src/api/api_datalog.cpp fixedpoint callback registration.

import (
	"github.com/glycerine/g3/pkg/ast"
	muzrel "github.com/glycerine/g3/pkg/muz/rel"
)

func (c *SMTContext) SetFixedpointExternalRelationCallbacks(callbacks muzrel.ExternalRelationCallbacks) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.SetFixedpointExternalRelationCallbacks(callbacks)
}

func (c *SMTContext) NewFixedpointExternalRelation(sorts []ast.ASTHandle) (*muzrel.ExternalRelation, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.NewFixedpointExternalRelation(sorts)
}
