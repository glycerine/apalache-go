package sls

// Source: src/tactic/sls/sls_tactic.h factory service binding.

import (
	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func QFBVConfigFromSolver(config smtcore.SMTSolverConfig) (QFBVConfig, error) {
	context, globals := config.Params, config.Globals
	services := config.ModelEvaluator
	if context == nil || globals == nil || globals.Context != context || services.Context != context || services.Globals != globals || services.Memory == nil || services.Cooperation == nil {
		return QFBVConfig{}, uparams.ErrParamsOwner
	}
	return QFBVConfig{Context: context, Globals: globals, ModelEvaluator: services}, nil
}
