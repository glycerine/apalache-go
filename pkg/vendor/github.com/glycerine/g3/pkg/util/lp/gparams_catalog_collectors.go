package lp

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

// Source: src/util/lp/lp_params.pyg
func CollectParamDescrs(d *uparams.ParamDescrs) error {
	return catalogCollectLpParams(d)
}

func catalogCollectLpParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "rep_freq", uparams.ParamUInt, "the report frequency, in how many iterations print the cost and other info ", "0", "lp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "min", uparams.ParamBool, "minimize cost", "false", "lp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "print_stats", uparams.ParamBool, "print statistic", "false", "lp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "simplex_strategy", uparams.ParamUInt, "simplex strategy for the solver", "0", "lp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bprop_on_pivoted_rows", uparams.ParamBool, "propagate bounds on rows changed by the pivot operation", "true", "lp"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectLpParams", catalogCollectLpParams)
}
