package cmd_context

// Source: src/api/api_tactic.cpp Z3_get_num_tactics/Z3_get_tactic_name and
// the source-ordered tactic factories dispatched by tactic_commands.go. The slice
// lists the retained Go factories in the native registration order.

import "github.com/glycerine/g3/pkg/arena"

var builtinTacticNames = [...]string{
	"ackermannize_bv", "subpaving", "horn", "horn-simplify", "nlsat", "qfnra-nlsat", "nlqsat", "qe-light",
	"qe-sat", "qe", "qsat", "qe2", "qe_rec", "sat", "sat-preprocess", "ctx-solver-simplify", "smt",
	"unit-subsume-simplify", "aig", "add-bounds", "card2bv", "degree-shift", "diff-neq", "elim01", "eq2bv",
	"factor", "fix-dl-var", "fm", "lia2card", "lia2pb", "nla2bv", "normalize-bounds", "pb2bv",
	"propagate-ineqs", "purify-arith", "recover-01", "bit-blast", "bv1-blast", "bv_bound_chk", "propagate-bv-bounds", "propagate-bv-bounds-new",
	"reduce-bv-size", "bvarray2uf", "dt2bv", "elim-small-bv", "max-bv-sharing", "blast-term-ite", "cofactor-term-ite", "collect-statistics",
	"ctx-simplify", "der", "distribute-forall", "dom-simplify", "elim-term-ite", "elim-uncnstr", "injectivity", "snf",
	"nnf", "occf", "pb-preprocess", "propagate-values", "reduce-args", "simplify", "elim-and", "solve-eqs",
	"split-clause", "symmetry-reduce", "tseitin-cnf", "tseitin-cnf-core", "fpa2bv", "qffp", "qffpbv", "qffplra",
	"nl-purify", "default", "sine-filter", "qfbv-sls", "nra", "qfaufbv", "qfauflia", "qfbv",
	"qfidl", "qflia", "qflra", "qfnia", "qfnra", "qfuf", "qfufbv", "qfufbv_ackr",
	"qfufnra", "ufnia", "uflra", "auflia", "auflira", "aufnira", "lra", "lia",
	"lira", "skip", "fail", "fail-if-undecided", "macro-finder", "quasi-macros", "ufbv-rewriter", "bv",
	"ufbv",
}

// IsBuiltinTactic reports whether name has a retained concrete factory.
func IsBuiltinTactic(name string) bool {
	for _, registered := range builtinTacticNames {
		if registered == name {
			return true
		}
	}
	return false
}

// BuiltinTacticCount reports the number of registered source tactics.
func BuiltinTacticCount() uint32 { return uint32(len(builtinTacticNames)) }

// BuiltinTacticName returns the source-ordered name at index.
func BuiltinTacticName(index uint32) (string, error) {
	if uint64(index) >= uint64(len(builtinTacticNames)) {
		return "", arena.ErrBounds
	}
	return builtinTacticNames[index], nil
}

// BuiltinTacticNames returns a caller-owned registry snapshot.
func BuiltinTacticNames() []string {
	result := make([]string, len(builtinTacticNames))
	copy(result, builtinTacticNames[:])
	return result
}
