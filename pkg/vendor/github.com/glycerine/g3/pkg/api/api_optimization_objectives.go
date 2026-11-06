package api

// Source: src/api/api_opt.cpp:379-393, Z3_optimize_get_objectives.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) OptimizationObjectiveCount() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return c.command.OptimizationObjectiveCount()
}

// OptimizationObjectives returns independently owning references in scoped
// insertion order. The caller must release every reference with Manager.DelRef.
func (c *SMTContext) OptimizationObjectives() ([]ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.OptimizationObjectives()
}
