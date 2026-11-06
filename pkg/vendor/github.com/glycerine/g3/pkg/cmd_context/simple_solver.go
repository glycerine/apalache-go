package cmd_context

// Source: src/api/api_solver.cpp init_solver_core and Z3_mk_simple_solver.

import (
	"errors"

	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
)

// NewSimpleSolver constructs an independent direct SMT solver over this
// context's manager and a snapshot of its current parameters.
func (c *Context) NewSimpleSolver() (result solver.Solver, resultErr error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	var proofs, models, cores bool
	if err = c.parameters.GetSolverParams(manager, &c.params, &proofs, &models, &cores); err != nil {
		return nil, err
	}
	direct, err := smtcore.NewSMTSolver(manager, &c.params, c.logic, c.config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, direct.Close())
			result = nil
		}
	}()
	if err = direct.SetProduceModels(models); err != nil {
		return nil, err
	}
	return direct, nil
}
