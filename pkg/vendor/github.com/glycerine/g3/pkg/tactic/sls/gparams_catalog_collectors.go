package sls

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

// Source: src/tactic/sls/sls_params.pyg
func catalogCollectSlsParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "max_memory", uparams.ParamUInt, "maximum amount of memory in megabytes", "4294967295", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "max_restarts", uparams.ParamUInt, "maximum number of restarts", "4294967295", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "walksat", uparams.ParamBool, "use walksat assertion selection (instead of gsat)", "true", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "walksat_ucb", uparams.ParamBool, "use bandit heuristic for walksat assertion selection (instead of random)", "true", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "walksat_ucb_constant", uparams.ParamDouble, "the ucb constant c in the term score + c * f(touched)", "20.0", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "walksat_ucb_init", uparams.ParamBool, "initialize total ucb touched to formula size", "false", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "walksat_ucb_forget", uparams.ParamDouble, "scale touched by this factor every base restart interval", "1.0", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "walksat_ucb_noise", uparams.ParamDouble, "add noise 0 <= 256 * ucb_noise to ucb score for assertion selection", "0.0002", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "walksat_repick", uparams.ParamBool, "repick assertion if randomizing in local minima", "true", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "scale_unsat", uparams.ParamDouble, "scale score of unsat expressions by this factor", "0.5", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "paws_init", uparams.ParamUInt, "initial/minimum assertion weights", "40", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "paws_sp", uparams.ParamUInt, "smooth assertion weights with probability paws_sp / 1024", "52", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "wp", uparams.ParamUInt, "random walk with probability wp / 1024", "100", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "vns_mc", uparams.ParamUInt, "in local minima, try Monte Carlo sampling vns_mc many 2-bit-flips per bit", "0", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "vns_repick", uparams.ParamBool, "in local minima, try picking a different assertion (only for walksat)", "false", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "restart_base", uparams.ParamUInt, "base restart interval given by moves per run", "100", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "restart_init", uparams.ParamBool, "initialize to 0 or random value (= 1) after restart", "false", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "early_prune", uparams.ParamBool, "use early pruning for score prediction", "true", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "random_offset", uparams.ParamBool, "use random offset for candidate evaluation", "true", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "rescore", uparams.ParamBool, "rescore/normalize top-level score every base restart interval", "true", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "track_unsat", uparams.ParamBool, "keep a list of unsat assertions as done in SAT - currently disabled internally", "false", "sls"); e != nil {
		return e
	}
	if e := catalogInsert(d, "random_seed", uparams.ParamUInt, "random seed", "0", "sls"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectSlsParams", catalogCollectSlsParams)
}
