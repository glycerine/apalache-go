package realclosure

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

// Source: src/math/realclosure/rcf_params.pyg
func catalogCollectRcfParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "use_prem", uparams.ParamBool, "use pseudo-remainder instead of remainder when computing GCDs and Sturm-Tarski sequences", "true", "rcf"); e != nil {
		return e
	}
	if e := catalogInsert(d, "clean_denominators", uparams.ParamBool, "clean denominators before root isolation", "true", "rcf"); e != nil {
		return e
	}
	if e := catalogInsert(d, "initial_precision", uparams.ParamUInt, "a value k that is the initial interval size (as 1/2^k) when creating transcendentals and approximated division", "24", "rcf"); e != nil {
		return e
	}
	if e := catalogInsert(d, "inf_precision", uparams.ParamUInt, "a value k that is the initial interval size (i.e., (0, 1/2^l)) used as an approximation for infinitesimal values", "24", "rcf"); e != nil {
		return e
	}
	if e := catalogInsert(d, "max_precision", uparams.ParamUInt, "during sign determination we switch from interval arithmetic to complete methods when the interval size is less than 1/2^k, where k is the max_precision", "128", "rcf"); e != nil {
		return e
	}
	if e := catalogInsert(d, "lazy_algebraic_normalization", uparams.ParamBool, "during sturm-seq and square-free polynomial computations, only normalize algebraic polynomial expressions when the definining polynomial is monic", "true", "rcf"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectRcfParams", catalogCollectRcfParams)
}
