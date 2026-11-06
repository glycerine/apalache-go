package opt

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

// Source: src/opt/opt_params.pyg
func catalogCollectOptParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "optsmt_engine", uparams.ParamSymbol, "select optimization engine: 'basic', 'farkas', 'symba'", "basic", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "maxsat_engine", uparams.ParamSymbol, "select engine for maxsat: 'core_maxsat', 'wmax', 'maxres', 'pd-maxres'", "maxres", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "priority", uparams.ParamSymbol, "select how to priortize objectives: 'lex' (lexicographic), 'pareto', or 'box'", "lex", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "dump_benchmarks", uparams.ParamBool, "dump benchmarks for profiling", "false", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "solution_prefix", uparams.ParamSymbol, "path prefix to dump intermediary, but non-optimal, solutions", "", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "timeout", uparams.ParamUInt, "timeout (in milliseconds) (UINT_MAX and 0 mean no timeout)", "4294967295", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "rlimit", uparams.ParamUInt, "resource limit (0 means no limit)", "0", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "enable_sls", uparams.ParamBool, "enable SLS tuning during weighted maxsast", "false", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "enable_sat", uparams.ParamBool, "enable the new SAT core for propositional constraints", "true", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "elim_01", uparams.ParamBool, "eliminate 01 variables", "true", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pp.neat", uparams.ParamBool, "use neat (as opposed to less readable, but faster) pretty printer when displaying context", "true", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pb.compile_equality", uparams.ParamBool, "compile arithmetical equalities into pseudo-Boolean equality (instead of two inequalites)", "false", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "maxres.hill_climb", uparams.ParamBool, "give preference for large weight cores", "true", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "maxres.add_upper_bound_block", uparams.ParamBool, "restict upper bound with constraint", "false", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "maxres.max_num_cores", uparams.ParamUInt, "maximal number of cores per round", "4294967295", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "maxres.max_core_size", uparams.ParamUInt, "break batch of generated cores if size reaches this number", "3", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "maxres.maximize_assignment", uparams.ParamBool, "find an MSS/MCS to improve current assignment", "false", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "maxres.max_correction_set_size", uparams.ParamUInt, "allow generating correction set constraints up to maximal size", "3", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "maxres.wmax", uparams.ParamBool, "use weighted theory solver to constrain upper bounds", "false", "opt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "maxres.pivot_on_correction_set", uparams.ParamBool, "reduce soft constraints if the current correction set is smaller than current core", "true", "opt"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectOptParams", catalogCollectOptParams)
}
