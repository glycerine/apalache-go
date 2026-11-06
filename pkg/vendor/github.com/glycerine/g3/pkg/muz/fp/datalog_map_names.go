package fp

// Source: src/muz/base/dl_context.cpp context::use_map_names and
// src/muz/base/fixedpoint_params.pyg datalog.use_map_names default.

import "errors"

// DatalogUseMapNames reports whether Datalog input uses mapped domain names.
func (e *PropositionalEngine) DatalogUseMapNames() (bool, error) {
	if err := e.open(); err != nil {
		return false, err
	}
	if e.params == nil || e.globals == nil {
		return true, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return false, err
	}
	value, err := view.Bool("datalog.use_map_names", true)
	return value, errors.Join(err, view.Close())
}
