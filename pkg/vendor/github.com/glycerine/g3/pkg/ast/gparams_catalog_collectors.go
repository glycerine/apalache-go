package ast

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

// Source: src/ast/pp_params.pyg
func catalogCollectPpParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "max_indent", uparams.ParamUInt, "max. indentation in pretty printer", "4294967295", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "max_num_lines", uparams.ParamUInt, "max. number of lines to be displayed in pretty printer", "4294967295", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "max_width", uparams.ParamUInt, "max. width in pretty printer", "80", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "max_ribbon", uparams.ParamUInt, "max. ribbon (width - indentation) in pretty printer", "80", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "max_depth", uparams.ParamUInt, "max. term depth (when pretty printing SMT2 terms/formulas)", "5", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "min_alias_size", uparams.ParamUInt, "min. size for creating an alias for a shared term (when pretty printing SMT2 terms/formulas)", "10", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "decimal", uparams.ParamBool, "pretty print real numbers using decimal notation (the output may be truncated). Z3 adds a ? if the value is not precise", "false", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "decimal_precision", uparams.ParamUInt, "maximum number of decimal places to be used when pp.decimal=true", "10", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv_literals", uparams.ParamBool, "use Bit-Vector literals (e.g, #x0F and #b0101) during pretty printing", "true", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "fp_real_literals", uparams.ParamBool, "use real-numbered floating point literals (e.g, +1.0p-1) during pretty printing", "false", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv_neg", uparams.ParamBool, "use bvneg when displaying Bit-Vector literals where the most significant bit is 1", "false", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "flat_assoc", uparams.ParamBool, "flat associative operators (when pretty printing SMT2 terms/formulas)", "true", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "fixed_indent", uparams.ParamBool, "use a fixed indentation for applications", "false", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "single_line", uparams.ParamBool, "ignore line breaks when true", "false", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bounded", uparams.ParamBool, "ignore characters exceeding max width", "false", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pretty_proof", uparams.ParamBool, "use slower, but prettier, printer for proofs", "false", "pp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "simplify_implies", uparams.ParamBool, "simplify nested implications for pretty printing", "true", "pp"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectPpParams", catalogCollectPpParams)
}
