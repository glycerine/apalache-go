package nlsat

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

// Source: src/nlsat/nlsat_params.pyg
func catalogCollectNlsatParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "max_memory", uparams.ParamUInt, "maximum amount of memory in megabytes", "4294967295", "nlsat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "lazy", uparams.ParamUInt, "how lazy the solver is.", "0", "nlsat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "reorder", uparams.ParamBool, "reorder variables.", "true", "nlsat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "simplify_conflicts", uparams.ParamBool, "simplify conflicts using equalities before resolving them in nlsat solver.", "true", "nlsat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "minimize_conflicts", uparams.ParamBool, "minimize conflicts", "false", "nlsat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "randomize", uparams.ParamBool, "randomize selection of a witness in nlsat.", "true", "nlsat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "max_conflicts", uparams.ParamUInt, "maximum number of conflicts.", "4294967295", "nlsat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "shuffle_vars", uparams.ParamBool, "use a random variable order.", "false", "nlsat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "inline_vars", uparams.ParamBool, "inline variables that can be isolated from equations", "false", "nlsat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "seed", uparams.ParamUInt, "random seed.", "0", "nlsat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "factor", uparams.ParamBool, "factor polynomials produced during conflict resolution.", "true", "nlsat"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectNlsatParams", catalogCollectNlsatParams)
}
