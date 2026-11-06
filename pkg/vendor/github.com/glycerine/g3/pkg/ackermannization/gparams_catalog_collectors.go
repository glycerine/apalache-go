package ackermannization

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

// Source: src/ackermannization/ackermannization_params.pyg
func catalogCollectAckermannizationParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "eager", uparams.ParamBool, "eagerly instantiate all congruence rules", "true", "ackermannization"); e != nil {
		return e
	}
	return nil
}

// Source: src/ackermannization/ackermannize_bv_tactic_params.pyg
func catalogCollectAckermannizeBvTacticParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "div0_ackermann_limit", uparams.ParamUInt, "a bound for number of congruence Ackermann lemmas for div0 modelling", "1000", "rewriter"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectAckermannizationParams", catalogCollectAckermannizationParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectAckermannizeBvTacticParams", catalogCollectAckermannizeBvTacticParams)
}
