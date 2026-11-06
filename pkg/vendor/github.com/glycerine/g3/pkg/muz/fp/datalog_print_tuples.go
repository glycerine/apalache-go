package fp

// Source: src/muz/base/dl_context.cpp context::output_tuples and
// src/muz/base/fixedpoint_params.pyg datalog.print.tuples default.

import "errors"

// DatalogPrintTuples reports whether tuples for output predicates should be printed.
func (e *PropositionalEngine) DatalogPrintTuples() (bool, error) {
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
	value, err := view.Bool("datalog.print.tuples", true)
	return value, errors.Join(err, view.Close())
}
