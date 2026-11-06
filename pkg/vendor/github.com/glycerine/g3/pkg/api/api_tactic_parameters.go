package api

// Source: src/api/api_tactic.cpp:356-376.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/params"
)

// ParameterDescriptions returns an independently owned copy of the
// descriptions collected by this concrete tactic.
func (t *APITactic) ParameterDescriptions() (result *ParameterDescriptions, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	globals, err := t.owner.command.GlobalParameters()
	if err != nil {
		return nil, err
	}
	value, err := params.NewParamDescrs(globals.Context.Symbols(), params.ParamDescrsConfig{Budget: globals.Context.Budget()})
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, value.Close())
		}
	}()
	if err = t.tactic.CollectParamDescrs(value); err != nil {
		return nil, err
	}
	return &ParameterDescriptions{owner: t.owner, value: value, symbols: globals.Context.Symbols()}, nil
}
