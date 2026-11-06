package solver

// Parameter collectors remain with the C++ definitions they translate.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func catalogInsert(d *uparams.ParamDescrs, key string, kind uparams.ParamKind, descr, def, module string) error {
	if module == "" {
		return d.InsertName(key, kind, &descr, &def, nil)
	}
	return d.InsertName(key, kind, &descr, &def, &module)
}

// Source: src/solver/combined_solver_params.pyg
func catalogCollectCombinedSolverParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "solver2_timeout", uparams.ParamUInt, "fallback to solver 1 after timeout even when in incremental model", "4294967295", "combined_solver"); e != nil {
		return e
	}
	if e := catalogInsert(d, "ignore_solver1", uparams.ParamBool, "if true, solver 2 is always used", "false", "combined_solver"); e != nil {
		return e
	}
	if e := catalogInsert(d, "solver2_unknown", uparams.ParamUInt, "what should be done when solver 2 returns unknown: 0 - just return unknown, 1 - execute solver 1 if quantifier free problem, 2 - execute solver 1", "1", "combined_solver"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectCombinedSolverParams", catalogCollectCombinedSolverParams)
}
