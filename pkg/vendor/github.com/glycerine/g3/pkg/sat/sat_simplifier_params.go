package sat

// Source: src/sat/sat_simplifier.cpp:1503–1522; src/sat/sat_simplifier_params.pyg.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// Keys correspond, in source order, to subsumed, subsumption resolution,
// elim literals, elim bool vars, and elim blocked clauses.

func (s *satSimplifier) collectStatistics(st *statistics.Statistics, keys [5]statistics.Key) error {
	if e := s.open(); e != nil {
		return e
	}
	values := [5]uint32{s.numSubsumed, s.numSubRes, s.numElimLits, s.numElimVars, s.numBlocked}
	for i, v := range values {
		st.UpdateUint(keys[i], v)
	}
	return nil
}
func collectSimplifierParamDescrs(d *uparams.ParamDescrs) error {
	for _, x := range [17]struct {
		name             string
		kind             uparams.ParamKind
		description, def string
	}{
		{"elim_blocked_clauses", uparams.ParamBool, "eliminate blocked clauses", "false"},
		{"elim_blocked_clauses_at", uparams.ParamUInt, "eliminate blocked clauses only once at the given simplification round", "2"},
		{"blocked_clause_limit", uparams.ParamUInt, "maximum number of literals visited during blocked clause elimination", "100000000"},
		{"resolution", uparams.ParamBool, "eliminate boolean variables using resolution", "true"},
		{"resolution.limit", uparams.ParamUInt, "approx. maximum number of literals visited during variable elimination", "500000000"},
		{"resolution.occ_cutoff", uparams.ParamUInt, "first cutoff (on number of positive/negative occurrences) for Boolean variable elimination", "10"},
		{"resolution.occ_cutoff_range1", uparams.ParamUInt, "second cutoff (number of positive/negative occurrences) for Boolean variable elimination, for problems containing less than res_cls_cutoff1 clauses", "8"},
		{"resolution.occ_cutoff_range2", uparams.ParamUInt, "second cutoff (number of positive/negative occurrences) for Boolean variable elimination, for problems containing more than res_cls_cutoff1 and less than res_cls_cutoff2", "5"},
		{"resolution.occ_cutoff_range3", uparams.ParamUInt, "second cutoff (number of positive/negative occurrences) for Boolean variable elimination, for problems containing more than res_cls_cutoff2", "3"},
		{"resolution.lit_cutoff_range1", uparams.ParamUInt, "second cutoff (total number of literals) for Boolean variable elimination, for problems containing less than res_cls_cutoff1 clauses", "700"},
		{"resolution.lit_cutoff_range2", uparams.ParamUInt, "second cutoff (total number of literals) for Boolean variable elimination, for problems containing more than res_cls_cutoff1 and less than res_cls_cutoff2", "400"},
		{"resolution.lit_cutoff_range3", uparams.ParamUInt, "second cutoff (total number of literals) for Boolean variable elimination, for problems containing more than res_cls_cutoff2", "300"},
		{"resolution.cls_cutoff1", uparams.ParamUInt, "limit1 - total number of problems clauses for the second cutoff of Boolean variable elimination", "100000000"},
		{"resolution.cls_cutoff2", uparams.ParamUInt, "limit2 - total number of problems clauses for the second cutoff of Boolean variable elimination", "700000000"},
		{"elim_vars", uparams.ParamBool, "enable variable elimination during simplification", "true"},
		{"subsumption", uparams.ParamBool, "eliminate subsumed clauses", "true"},
		{"subsumption.limit", uparams.ParamUInt, "approx. maximum number of literals visited during subsumption (and subsumption resolution)", "100000000"},
	} {
		if e := satConfigInsertDescriptor(d, x.name, x.kind, x.description, x.def); e != nil {
			return e
		}
	}
	return nil
}
