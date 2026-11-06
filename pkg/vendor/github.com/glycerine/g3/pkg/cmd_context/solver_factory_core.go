package cmd_context

// Source: src/api/api_solver.cpp init_solver_core.

import (
	"errors"

	"github.com/glycerine/g3/pkg/solver"
)

func (c *Context) newSolverFromFactory(factory solver.Factory, logic string) (result solver.Solver, resultErr error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, errors.New("command context: nil solver factory")
	}
	defer func() {
		if closeErr := factory.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, closeErr)
			if result != nil {
				resultErr = errors.Join(resultErr, result.Close())
				result = nil
			}
		}
	}()
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	var proofs, models, cores bool
	if err = c.parameters.GetSolverParams(manager, &c.params, &proofs, &models, &cores); err != nil {
		return nil, err
	}
	return factory.New(manager, &c.params, proofs, models, cores, logic)
}
