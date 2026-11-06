package api

// Source: src/api/api_datalog.cpp fixedpoint parameter descriptions and help.

import "errors"

// ParameterDescriptions returns an independently owned fixedpoint catalog.
func (f *APIFixedpoint) ParameterDescriptions() (*ParameterDescriptions, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	return (&SMTContext{smtContextState: f.owner}).ParameterDescriptions("fixedpoint")
}

// ParameterHelp renders the fixedpoint catalog in source parameter-help format.
func (f *APIFixedpoint) ParameterHelp() (text string, resultErr error) {
	d, err := f.ParameterDescriptions()
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, d.Close()) }()
	return d.value.Display(0, false, true)
}

// SetParameters replaces the API wrapper snapshot and copies the update into
// the cumulative fixedpoint engine snapshot.
func (f *APIFixedpoint) SetParameters(parameters *ParameterSet) (resultErr error) {
	if err := f.open(); err != nil {
		return err
	}
	if err := parameters.open(); err != nil {
		return err
	}
	if parameters.owner != f.owner {
		return ErrSMTContextOwner
	}
	descriptions, err := f.ParameterDescriptions()
	if err != nil {
		return err
	}
	if err = parameters.Validate(descriptions); err != nil {
		return errors.Join(err, descriptions.Close())
	}
	if err = descriptions.Close(); err != nil {
		return err
	}
	name := "timeout"
	hasTimeout, err := parameters.params.ContainsName(&parameters.value, &name)
	if err != nil {
		return err
	}
	var timeout uint32
	if hasTimeout {
		timeout, err = parameters.params.GetUIntName(&parameters.value, &name, 0)
		if err != nil {
			return err
		}
	}
	next, err := (&SMTContext{smtContextState: f.owner}).NewParameterSet()
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, next.Close())
		}
	}()
	if err = next.params.Append(&next.value, &parameters.value); err != nil {
		return err
	}
	engineNext, err := (&SMTContext{smtContextState: f.owner}).NewParameterSet()
	if err != nil {
		return err
	}
	enginePublished := false
	defer func() {
		if !enginePublished {
			resultErr = errors.Join(resultErr, engineNext.Close())
		}
	}()
	if f.engineParams != nil {
		if err = engineNext.params.Append(&engineNext.value, &f.engineParams.value); err != nil {
			return err
		}
	}
	if err = engineNext.params.Append(&engineNext.value, &parameters.value); err != nil {
		return err
	}
	if err = f.command.SetFixedpointParameters(&engineNext.value); err != nil {
		return err
	}
	old, oldEngine := f.params, f.engineParams
	f.params, f.engineParams = next, engineNext
	published, enginePublished = true, true
	f.timeout, f.hasTimeout = timeout, hasTimeout
	for _, prior := range []*ParameterSet{old, oldEngine} {
		if prior != nil {
			if closeErr := prior.Close(); closeErr != nil {
				f.retiredParams = append(f.retiredParams, prior)
				resultErr = errors.Join(resultErr, closeErr)
			}
		}
	}
	return resultErr
}
