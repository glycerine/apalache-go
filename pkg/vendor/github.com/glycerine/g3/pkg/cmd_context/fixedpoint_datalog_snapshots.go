package cmd_context

import (
	"github.com/glycerine/g3/pkg/ast"
	muzfp "github.com/glycerine/g3/pkg/muz/fp"
)

type FixedpointRelationSnapshot = muzfp.DatalogRelationSnapshot

func (c *Context) FixedpointRelationSnapshots(declarations []ast.ASTHandle) ([]FixedpointRelationSnapshot, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	return engine.DatalogRelationSnapshots(declarations)
}
