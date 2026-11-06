package cmd_context

// Source: src/api/api_datalog.cpp fixedpoint_from_stream rule transfer.

import "github.com/glycerine/g3/pkg/ast"

// FixedpointNamedRules returns owning formula references and their names in
// insertion order. The caller releases the references through Manager.DelRef.
func (c *Context) FixedpointNamedRules() ([]ast.ASTRef, []string, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, nil, err
	}
	return engine.NamedRules()
}
