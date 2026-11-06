package api

// Source: src/api/api_tactic.cpp:449-458 parameter validation prefix.

import "errors"

func (t *APITactic) validateParameters(parameters *ParameterSet) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if err := parameters.open(); err != nil {
		return err
	}
	if t.owner != parameters.owner {
		return ErrSMTContextOwner
	}
	descriptions, err := t.ParameterDescriptions()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, descriptions.Close()) }()
	return parameters.Validate(descriptions)
}
