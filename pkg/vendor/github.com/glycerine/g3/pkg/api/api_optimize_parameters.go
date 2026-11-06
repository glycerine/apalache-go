package api

// Source: src/api/api_opt.cpp:179-189.

import "errors"

func (o *APIOptimize) SetParameters(parameters *ParameterSet) error {
	if err := o.open(); err != nil {
		return err
	}
	if err := parameters.open(); err != nil {
		return err
	}
	if parameters.owner != o.owner {
		return ErrSMTContextOwner
	}
	descriptions, err := o.ParameterDescriptions()
	if err != nil {
		return err
	}
	if err = parameters.Validate(descriptions); err != nil {
		return errors.Join(err, descriptions.Close())
	}
	if err = descriptions.Close(); err != nil {
		return err
	}
	return o.command.UpdateOptimizationParameters(&parameters.value)
}
