package api

// Source: src/api/api_datalog.cpp query scopes and fixedpoint parameter snapshot.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (f *APIFixedpoint) effectiveQueryTimeout() (uint32, error) {
	if f.hasTimeout {
		return f.timeout, nil
	}
	return (&SMTContext{smtContextState: f.owner}).contextTimeout()
}

// executeQuery copies the cumulative engine snapshot but selects the timeout
// from the latest replacement parameter set, falling back to the current parent.
func (f *APIFixedpoint) executeQuery(limited bool, run func(*uparams.ParamsRef) (lbool.LBool, error)) (status lbool.LBool, resultErr error) {
	context := &SMTContext{smtContextState: f.owner}
	timeout, err := f.effectiveQueryTimeout()
	if err != nil {
		return lbool.Undef, err
	}
	queryParams, err := context.NewParameterSet()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, queryParams.Close()) }()
	if f.engineParams != nil {
		if err = queryParams.params.Append(&queryParams.value, &f.engineParams.value); err != nil {
			return lbool.Undef, err
		}
	}
	if err = queryParams.SetUInt("timeout", timeout); err != nil {
		return lbool.Undef, err
	}
	if limited {
		scope, e := context.beginFixedpointResourceLimit()
		if e != nil {
			return lbool.Undef, e
		}
		defer scope.Close()
	}
	return run(&queryParams.value)
}

func (c *SMTContext) beginFixedpointResourceLimit() (control.LimitScope, error) {
	text, found, err := c.command.GetOption("rlimit")
	if err != nil {
		return control.LimitScope{}, err
	}
	if !found {
		return control.LimitScope{}, ErrSMTContextOwner
	}
	limit, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return control.LimitScope{}, err
	}
	manager, err := c.Manager()
	if err != nil {
		return control.LimitScope{}, err
	}
	return control.NewLimitScope(manager.ResourceLimit(), uint32(limit))
}
