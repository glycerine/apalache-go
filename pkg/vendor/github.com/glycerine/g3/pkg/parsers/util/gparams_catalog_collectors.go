package util

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

// Source: src/parsers/util/parser_params.pyg
func catalogCollectParserParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "ignore_user_patterns", uparams.ParamBool, "ignore patterns provided by the user", "false", "parser"); e != nil {
		return e
	}
	if e := catalogInsert(d, "ignore_bad_patterns", uparams.ParamBool, "ignore malformed patterns", "true", "parser"); e != nil {
		return e
	}
	if e := catalogInsert(d, "error_for_visual_studio", uparams.ParamBool, "display error messages in Visual Studio format", "false", "parser"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectParserParams", catalogCollectParserParams)
}
