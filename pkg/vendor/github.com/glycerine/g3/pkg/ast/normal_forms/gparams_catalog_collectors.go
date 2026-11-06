package normal_forms

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

// Source: src/ast/normal_forms/nnf.h
func catalogCollectNnf(d *uparams.ParamDescrs) error { return catalogCollectNnfParams(d) }

// Source: src/ast/normal_forms/nnf_params.pyg
func catalogCollectNnfParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "max_memory", uparams.ParamUInt, "maximum amount of memory in megabytes", "4294967295", "nnf"); e != nil {
		return e
	}
	if e := catalogInsert(d, "sk_hack", uparams.ParamBool, "hack for VCC", "false", "nnf"); e != nil {
		return e
	}
	if e := catalogInsert(d, "mode", uparams.ParamSymbol, "NNF translation mode: skolem (skolem normal form), quantifiers (skolem normal form + quantifiers in NNF), full", "skolem", "nnf"); e != nil {
		return e
	}
	if e := catalogInsert(d, "ignore_labels", uparams.ParamBool, "remove/ignore labels in the input formula, this option is ignored if proofs are enabled", "false", "nnf"); e != nil {
		return e
	}
	if e := catalogInsert(d, "skolemize", uparams.ParamBool, "skolemize (existential force) quantifiers", "true", "nnf"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectNnf", catalogCollectNnf)
	uparams.RegisterGlobalParamsCollector("catalogCollectNnfParams", catalogCollectNnfParams)
}
