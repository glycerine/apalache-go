package api

// Source: src/api/api_opt.cpp:183-202.

import "errors"

func (o *APIOptimize) Model() (*APIModel, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	core, err := o.command.OptimizationModel()
	if err != nil {
		return nil, err
	}
	result, err := newAPIModel(o.owner, core, true)
	if err != nil {
		return nil, errors.Join(err, core.DecRef())
	}
	return result, nil
}
