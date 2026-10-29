package sat

// Source: src/sat/sat_params.pyg; src/sat/sat_config.cpp.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Actual sat::config::collect_param_descrs delegates to the generated collector.
// These literal calls preserve sat_params.pyg insertion order, descriptions,
// defaults and module bytes. There is no intermediate metadata slice.
func collectSATConfigParamDescrs(d *uparams.ParamDescrs) error {
	if d == nil {
		return uparams.ErrParamDescrsPrecondition
	}
	if e := satConfigInsertDescriptor(d, "max_memory", uparams.ParamUInt, "maximum amount of memory in megabytes", "4294967295"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "phase", uparams.ParamSymbol, "phase selection strategy: always_false, always_true, caching, random", "caching"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "phase.caching.on", uparams.ParamUInt, "phase caching on period (in number of conflicts)", "400"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "phase.caching.off", uparams.ParamUInt, "phase caching off period (in number of conflicts)", "100"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "restart", uparams.ParamSymbol, "restart strategy: luby or geometric", "luby"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "restart.initial", uparams.ParamUInt, "initial restart (number of conflicts)", "100"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "restart.max", uparams.ParamUInt, "maximal number of restarts.", "4294967295"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "restart.factor", uparams.ParamDouble, "restart increment factor for geometric strategy", "1.5"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "random_freq", uparams.ParamDouble, "frequency of random case splits", "0.01"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "random_seed", uparams.ParamUInt, "random seed", "0"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "burst_search", uparams.ParamUInt, "number of conflicts before first global simplification", "100"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "max_conflicts", uparams.ParamUInt, "maximum number of conflicts", "4294967295"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "gc", uparams.ParamSymbol, "garbage collection strategy: psm, glue, glue_psm, dyn_psm", "glue_psm"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "gc.initial", uparams.ParamUInt, "learned clauses garbage collection frequence", "20000"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "gc.increment", uparams.ParamUInt, "increment to the garbage collection threshold", "500"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "gc.small_lbd", uparams.ParamUInt, "learned clauses with small LBD are never deleted (only used in dyn_psm)", "3"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "gc.k", uparams.ParamUInt, "learned clauses that are inactive for k gc rounds are permanently deleted (only used in dyn_psm)", "7"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "minimize_lemmas", uparams.ParamBool, "minimize learned clauses", "true"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "dyn_sub_res", uparams.ParamBool, "dynamic subsumption resolution for minimizing learned clauses", "true"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "core.minimize", uparams.ParamBool, "minimize computed core", "false"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "core.minimize_partial", uparams.ParamBool, "apply partial (cheap) core minimization", "false"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "parallel_threads", uparams.ParamUInt, "number of parallel threads to use", "1"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "dimacs.core", uparams.ParamBool, "extract core from DIMACS benchmarks", "false"); e != nil {
		return e
	}
	if e := satConfigInsertDescriptor(d, "dimacs.display", uparams.ParamBool, "display SAT instance in DIMACS format and return unknown instead of solving", "false"); e != nil {
		return e
	}
	return nil
}

func satConfigInsertDescriptor(d *uparams.ParamDescrs, name string, kind uparams.ParamKind, description, defaultValue string) error {
	module := "sat"
	return d.InsertName(name, kind, &description, &defaultValue, &module)
}

func init() {
	uparams.RegisterGlobalParamsCollector("CollectSATConfigParamDescrs", collectSATConfigParamDescrs)
}
