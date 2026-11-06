package api

// Source: src/api/api_solver.cpp:206-248.

import (
	"errors"

	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (s *APISimpleSolver) ParameterDescriptions() (*ParameterDescriptions, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	globals, err := s.owner.command.GlobalParameters()
	if err != nil {
		return nil, err
	}
	value, err := uparams.NewParamDescrs(globals.Context.Symbols(), uparams.ParamDescrsConfig{Budget: globals.Context.Budget()})
	if err != nil {
		return nil, err
	}
	if err = s.solver.CollectParamDescrs(value); err != nil {
		return nil, errors.Join(err, value.Close())
	}
	if err = smtcore.CollectSolverParamDescriptions(value); err != nil {
		return nil, errors.Join(err, value.Close())
	}
	return &ParameterDescriptions{owner: s.owner, value: value, symbols: globals.Context.Symbols()}, nil
}

func (s *APISimpleSolver) SetParameters(parameters *ParameterSet) (resultErr error) {
	if err := s.open(); err != nil {
		return err
	}
	if err := parameters.open(); err != nil {
		return err
	}
	if parameters.owner != s.owner {
		return ErrSMTContextOwner
	}
	descriptions, err := s.ParameterDescriptions()
	if err != nil {
		return err
	}
	if err = parameters.Validate(descriptions); err != nil {
		_ = descriptions.Close()
		return err
	}
	if err = descriptions.Close(); err != nil {
		return err
	}
	merged, err := parameters.params.NewValue()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.params.Del(&merged)) }()
	current, err := s.solver.Params()
	if err != nil {
		return err
	}
	if err = parameters.params.Append(&merged, current); err != nil {
		return err
	}
	if err = parameters.params.Append(&merged, &parameters.value); err != nil {
		return err
	}
	name := "model"
	oldModels := true
	if s.explicitReady {
		oldModels, err = parameters.params.GetBoolName(&s.explicitParams, &name, true)
		if err != nil {
			return err
		}
	}
	models, err := parameters.params.GetBoolName(&parameters.value, &name, true)
	if err != nil {
		return err
	}
	if oldModels != models {
		if err = s.solver.SetProduceModels(models); err != nil {
			return err
		}
	}
	if err = s.solver.UpdateParams(&merged); err != nil {
		return err
	}
	if !s.explicitReady {
		s.explicitParams, err = parameters.params.NewValue()
		if err != nil {
			return err
		}
		s.explicitContext = parameters.params
		s.explicitReady = true
	}
	return parameters.params.Append(&s.explicitParams, &parameters.value)
}
