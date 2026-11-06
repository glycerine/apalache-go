package api

// Source: src/api/api_tactic.cpp:203-219.

import (
	"errors"

	"github.com/glycerine/g3/pkg/tactic"
)

// UsingParameters validates parameters against the concrete tactic metadata,
// copies them into a using-params tactic, and returns an independent owner.
func (t *APITactic) UsingParameters(parameters *ParameterSet) (result *APITactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if err := parameters.open(); err != nil {
		return nil, err
	}
	if t.owner != parameters.owner {
		return nil, ErrSMTContextOwner
	}
	descriptions, err := t.ParameterDescriptions()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, descriptions.Close()) }()
	if err = parameters.Validate(descriptions); err != nil {
		return nil, err
	}
	value, err := tactic.NewUsingParamsTactic(parameters.params, t.tactic, &parameters.value)
	if err != nil {
		return nil, err
	}
	return newAPITactic(t.owner, value)
}
