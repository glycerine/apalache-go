package cmd_context

// Source: src/opt/opt_context.cpp context::updt_params.

import uparams "github.com/glycerine/g3/pkg/util/params"

func (c *Context) UpdateOptimizationParameters(parameters *uparams.ParamsRef) error {
	if err := c.valid(); err != nil {
		return err
	}
	if parameters == nil {
		return uparams.ErrParamsOwner
	}
	if err := c.config.Params.Check(parameters); err != nil {
		return err
	}
	if err := c.config.Params.Append(&c.params, parameters); err != nil {
		return err
	}
	// opt_context::updt_params updates the solver already owned by an active
	// GIA Pareto engine. Other optimization paths construct their solvers on
	// the next check and read c.params there.
	if c.optimizationParetoBackend != nil {
		return c.optimizationParetoBackend.UpdateParams(&c.params)
	}
	return nil
}
