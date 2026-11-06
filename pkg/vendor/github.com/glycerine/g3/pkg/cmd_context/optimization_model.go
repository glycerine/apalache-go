package cmd_context

// Source: src/api/api_opt.cpp:161-177, Z3_optimize_get_model empty-model
// fallback when opt::context has no current model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/model"
)

func (c *Context) OptimizationModel() (*model.ModelCore, error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	state, err := c.parameters.State()
	if err != nil {
		return nil, err
	}
	if c.checked && state.Model {
		value, acquireErr := c.checkSatResult().AcquireModel()
		if acquireErr != nil {
			return nil, acquireErr
		}
		if value != nil {
			return value, nil
		}
	}
	empty, err := model.NewModel(manager)
	if err != nil {
		return nil, err
	}
	core := empty.Core()
	if err = core.IncRef(); err != nil {
		return nil, errors.Join(err, empty.Close())
	}
	return core, nil
}
