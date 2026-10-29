package sat

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

// Source: src/sat/sat_asymm_branch_params.pyg
func catalogCollectSatAsymmBranchParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "asymm_branch", uparams.ParamBool, "asymmetric branching", "true", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "asymm_branch.rounds", uparams.ParamUInt, "maximum number of rounds of asymmetric branching", "32", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "asymm_branch.limit", uparams.ParamUInt, "approx. maximum number of literals visited during asymmetric branching", "100000000", "sat"); e != nil {
		return e
	}
	return nil
}

// Source: src/sat/sat_scc_params.pyg
func catalogCollectSatSccParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "scc", uparams.ParamBool, "eliminate Boolean variables by computing strongly connected components", "true", "sat"); e != nil {
		return e
	}
	return nil
}

// Source: src/sat/sat_simplifier_params.pyg
func catalogCollectSatSimplifierParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "elim_blocked_clauses", uparams.ParamBool, "eliminate blocked clauses", "false", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "elim_blocked_clauses_at", uparams.ParamUInt, "eliminate blocked clauses only once at the given simplification round", "2", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "blocked_clause_limit", uparams.ParamUInt, "maximum number of literals visited during blocked clause elimination", "100000000", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "resolution", uparams.ParamBool, "eliminate boolean variables using resolution", "true", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "resolution.limit", uparams.ParamUInt, "approx. maximum number of literals visited during variable elimination", "500000000", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "resolution.occ_cutoff", uparams.ParamUInt, "first cutoff (on number of positive/negative occurrences) for Boolean variable elimination", "10", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "resolution.occ_cutoff_range1", uparams.ParamUInt, "second cutoff (number of positive/negative occurrences) for Boolean variable elimination, for problems containing less than res_cls_cutoff1 clauses", "8", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "resolution.occ_cutoff_range2", uparams.ParamUInt, "second cutoff (number of positive/negative occurrences) for Boolean variable elimination, for problems containing more than res_cls_cutoff1 and less than res_cls_cutoff2", "5", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "resolution.occ_cutoff_range3", uparams.ParamUInt, "second cutoff (number of positive/negative occurrences) for Boolean variable elimination, for problems containing more than res_cls_cutoff2", "3", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "resolution.lit_cutoff_range1", uparams.ParamUInt, "second cutoff (total number of literals) for Boolean variable elimination, for problems containing less than res_cls_cutoff1 clauses", "700", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "resolution.lit_cutoff_range2", uparams.ParamUInt, "second cutoff (total number of literals) for Boolean variable elimination, for problems containing more than res_cls_cutoff1 and less than res_cls_cutoff2", "400", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "resolution.lit_cutoff_range3", uparams.ParamUInt, "second cutoff (total number of literals) for Boolean variable elimination, for problems containing more than res_cls_cutoff2", "300", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "resolution.cls_cutoff1", uparams.ParamUInt, "limit1 - total number of problems clauses for the second cutoff of Boolean variable elimination", "100000000", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "resolution.cls_cutoff2", uparams.ParamUInt, "limit2 - total number of problems clauses for the second cutoff of Boolean variable elimination", "700000000", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "elim_vars", uparams.ParamBool, "enable variable elimination during simplification", "true", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "subsumption", uparams.ParamBool, "eliminate subsumed clauses", "true", "sat"); e != nil {
		return e
	}
	if e := catalogInsert(d, "subsumption.limit", uparams.ParamUInt, "approx. maximum number of literals visited during subsumption (and subsumption resolution)", "100000000", "sat"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectSatAsymmBranchParams", catalogCollectSatAsymmBranchParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectSatSccParams", catalogCollectSatSccParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectSatSimplifierParams", catalogCollectSatSimplifierParams)
}
