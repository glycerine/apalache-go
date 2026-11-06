package cmd_context

// Source: src/cmd_context/cmd_context.cpp display_dimacs.

import (
	"errors"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

// DisplayDIMACS temporarily enables SAT DIMACS output and runs the active
// incremental solver, restoring the source command's false setting on exit.
func (c *Context) DisplayDIMACS() (resultErr error) {
	if err := c.valid(); err != nil {
		return err
	}
	if c.solver == nil {
		return nil
	}
	if err := c.config.Globals.SourceSetName("sat.dimacs.display", "true"); err != nil {
		return err
	}
	params := c.config.Params.EmptyValue()
	defer func() {
		resultErr = errors.Join(resultErr, c.config.Globals.SourceSetName("sat.dimacs.display", "false"))
		resultErr = errors.Join(resultErr, c.solver.UpdateParams(&params))
		if params != (uparams.ParamsRef{}) {
			resultErr = errors.Join(resultErr, c.config.Params.Del(&params))
		}
	}()
	if err := c.solver.UpdateParams(&params); err != nil {
		return err
	}
	status, err := c.solver.CheckSat(nil)
	if err != nil {
		return err
	}
	if c.checked && c.tacticResult == nil && c.retainedSolverResult == nil {
		c.status = status
	}
	return nil
}
