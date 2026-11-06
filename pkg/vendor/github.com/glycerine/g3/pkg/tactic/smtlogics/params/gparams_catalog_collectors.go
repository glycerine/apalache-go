package params

// Parameter collectors remain below the SMT strategy factories, so their
// source initialization does not pull the backend into its own dependencies.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func catalogInsert(d *uparams.ParamDescrs, key string, kind uparams.ParamKind, descr, def, module string) error {
	if module == "" {
		return d.InsertName(key, kind, &descr, &def, nil)
	}
	return d.InsertName(key, kind, &descr, &def, &module)
}

// Source: src/tactic/smtlogics/qfufbv_tactic_params.pyg
func catalogCollectQfufbvTacticParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "sat_backend", uparams.ParamBool, "use SAT rather than SMT in qfufbv_ackr_tactic", "false", "ackermannization"); e != nil {
		return e
	}
	if e := catalogInsert(d, "inc_sat_backend", uparams.ParamBool, "use incremental SAT", "false", "ackermannization"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectQfufbvTacticParams", catalogCollectQfufbvTacticParams)
}
