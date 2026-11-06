package model

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

// Source: src/model/model_evaluator_params.pyg
func catalogCollectModelEvaluatorParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "max_memory", uparams.ParamUInt, "maximum amount of memory in megabytes", "4294967295", "model_evaluator"); e != nil {
		return e
	}
	if e := catalogInsert(d, "max_steps", uparams.ParamUInt, "maximum number of steps", "4294967295", "model_evaluator"); e != nil {
		return e
	}
	if e := catalogInsert(d, "completion", uparams.ParamBool, "assigns an interptetation to symbols that do not have one in the current model, when evaluating expressions in the current model", "false", "model_evaluator"); e != nil {
		return e
	}
	if e := catalogInsert(d, "cache", uparams.ParamBool, "cache intermediate results in the model evaluator", "true", "model_evaluator"); e != nil {
		return e
	}
	if e := catalogInsert(d, "array_equalities", uparams.ParamBool, "evaluate array equalities", "true", "model_evaluator"); e != nil {
		return e
	}
	return nil
}

// Source: src/model/model_params.pyg
func catalogCollectModelParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "partial", uparams.ParamBool, "enable/disable partial function interpretations", "false", "model"); e != nil {
		return e
	}
	if e := catalogInsert(d, "v1", uparams.ParamBool, "use Z3 version 1.x pretty printer", "false", "model"); e != nil {
		return e
	}
	if e := catalogInsert(d, "v2", uparams.ParamBool, "use Z3 version 2.x (x <= 16) pretty printer", "false", "model"); e != nil {
		return e
	}
	if e := catalogInsert(d, "compact", uparams.ParamBool, "try to compact function graph (i.e., function interpretations that are lookup tables)", "false", "model"); e != nil {
		return e
	}
	if e := catalogInsert(d, "completion", uparams.ParamBool, "enable/disable model completion", "false", "model"); e != nil {
		return e
	}
	return nil
}

// ModelEvaluatorParameterDescriptions is model_evaluator::get_param_descrs.
func ModelEvaluatorParameterDescriptions(d *uparams.ParamDescrs) error {
	return catalogCollectModelEvaluatorParams(d)
}

// ModelParameterDescriptions is model_params::collect_param_descrs.
func ModelParameterDescriptions(d *uparams.ParamDescrs) error {
	return catalogCollectModelParams(d)
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectModelEvaluatorParams", catalogCollectModelEvaluatorParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectModelParams", catalogCollectModelParams)
}
