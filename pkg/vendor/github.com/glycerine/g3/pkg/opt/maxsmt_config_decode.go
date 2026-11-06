package opt

// Source: src/opt/maxres.cpp updt_params and maxsmt.cpp engine selection.

import "errors"

func loadMaxSMTConfig(s *ArithmeticSolver) (config maxSMTConfig, resultErr error) {
	config.satCoreEnabled = s.satCoreEnabled
	globals := s.config.Globals
	if globals == nil {
		globals = s.config.ModelEvaluator.Globals
	}
	params, err := NewParams(s.config.Params, globals, s.config.ModelEvaluator.Params)
	if err != nil {
		return config, err
	}
	defer func() { resultErr = errors.Join(resultErr, params.Close()) }()
	engine, err := params.MaxSATEngine()
	if err != nil {
		return config, err
	}
	config.engine, err = s.manager.Symbols.String(engine)
	if err != nil {
		return config, err
	}
	if config.enableSLS, err = params.EnableSLS(); err != nil {
		return config, err
	}
	if config.hillClimb, err = params.MaxResHillClimb(); err != nil {
		return config, err
	}
	if config.addUpperBoundBlock, err = params.MaxResAddUpperBoundBlock(); err != nil {
		return config, err
	}
	if config.maxNumCores, err = params.MaxResMaxNumCores(); err != nil {
		return config, err
	}
	if config.maxCoreSize, err = params.MaxResMaxCoreSize(); err != nil {
		return config, err
	}
	if config.maximizeAssignment, err = params.MaxResMaximizeAssignment(); err != nil {
		return config, err
	}
	if config.maxCorrectionSetSize, err = params.MaxResMaxCorrectionSetSize(); err != nil {
		return config, err
	}
	if config.pivotOnCorrectionSet, err = params.MaxResPivotOnCorrectionSet(); err != nil {
		return config, err
	}
	return config, nil
}
