package smtcore

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

// Source: src/cmd_context/context_params.cpp
func catalogCollectContextParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "timeout", uparams.ParamUInt, "default timeout (in milliseconds) used for solvers", "4294967295", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "rlimit", uparams.ParamUInt, "default resource limit used for solvers. Unrestricted when set to 0.", "0", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "well_sorted_check", uparams.ParamBool, "type checker", "false", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "type_check", uparams.ParamBool, "type checker (alias for well_sorted_check)", "true", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "auto_config", uparams.ParamBool, "use heuristics to automatically select solver and configure it", "true", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "model_validate", uparams.ParamBool, "validate models produced by solvers", "false", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "dump_models", uparams.ParamBool, "dump models whenever check-sat returns sat", "false", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "trace", uparams.ParamBool, "trace generation for VCC", "false", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "trace_file_name", uparams.ParamString, "trace out file name (see option 'trace')", "z3.log", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "dot_proof_file", uparams.ParamString, "file in which to output graphical proofs", "proof.dot", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "debug_ref_count", uparams.ParamBool, "debug support for AST reference counting", "false", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "smtlib2_compliant", uparams.ParamBool, "enable/disable SMT-LIB 2.0 compliance", "false", ""); e != nil {
		return e
	}
	return catalogCollectContextSolverParams(d)
}

// Source: src/cmd_context/context_params.cpp
func catalogCollectContextSolverParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "proof", uparams.ParamBool, "proof generation, it must be enabled when the Z3 context is created", "false", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "model", uparams.ParamBool, "model generation for solvers, this parameter can be overwritten when creating a solver", "true", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "unsat_core", uparams.ParamBool, "unsat-core generation for solvers, this parameter can be overwritten when creating a solver, not every solver in Z3 supports unsat core generation", "false", ""); e != nil {
		return e
	}
	return nil
}

// Source: src/ast/pattern/pattern_inference_params_helper.pyg
func catalogCollectPatternInferenceParamsHelper(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "max_multi_patterns", uparams.ParamUInt, "when patterns are not provided, the prover uses a heuristic to infer them, this option sets the threshold on the number of extra multi-patterns that can be created; by default, the prover creates at most one multi-pattern when there is no unary pattern", "0", "pi"); e != nil {
		return e
	}
	if e := catalogInsert(d, "block_loop_patterns", uparams.ParamBool, "block looping patterns during pattern inference", "true", "pi"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith", uparams.ParamUInt, "0 - do not infer patterns with arithmetic terms, 1 - use patterns with arithmetic terms if there is no other pattern, 2 - always use patterns with arithmetic terms", "1", "pi"); e != nil {
		return e
	}
	if e := catalogInsert(d, "use_database", uparams.ParamBool, "use pattern database", "false", "pi"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith_weight", uparams.ParamUInt, "default weight for quantifiers where the only available pattern has nested arithmetic terms", "5", "pi"); e != nil {
		return e
	}
	if e := catalogInsert(d, "non_nested_arith_weight", uparams.ParamUInt, "default weight for quantifiers where the only available pattern has non nested arithmetic terms", "10", "pi"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pull_quantifiers", uparams.ParamBool, "pull nested quantifiers, if no pattern was found", "true", "pi"); e != nil {
		return e
	}
	if e := catalogInsert(d, "warnings", uparams.ParamBool, "enable/disable warning messages in the pattern inference module", "false", "pi"); e != nil {
		return e
	}
	return nil
}

// Source: src/interp/interp_params.pyg
func catalogCollectInterpParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "profile", uparams.ParamBool, "(INTERP) profile interpolation", "false", "interp"); e != nil {
		return e
	}
	if e := catalogInsert(d, "check", uparams.ParamBool, "(INTERP) check interpolants", "false", "interp"); e != nil {
		return e
	}
	return nil
}

// Source: src/smt/params/smt_params_helper.pyg
func catalogCollectSmtParamsHelper(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "auto_config", uparams.ParamBool, "automatically configure solver", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "logic", uparams.ParamSymbol, "logic used to setup the SMT solver", "", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "random_seed", uparams.ParamUInt, "random seed for the smt solver", "0", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "relevancy", uparams.ParamUInt, "relevancy propagation heuristic: 0 - disabled, 1 - relevancy is tracked by only affects quantifier instantiation, 2 - relevancy is tracked, and an atom is only asserted if it is relevant", "2", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "macro_finder", uparams.ParamBool, "try to find universally quantified formulas that can be viewed as macros", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "quasi_macros", uparams.ParamBool, "try to find universally quantified formulas that are quasi-macros", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "restricted_quasi_macros", uparams.ParamBool, "try to find universally quantified formulas that are restricted quasi-macros", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "ematching", uparams.ParamBool, "E-Matching based quantifier instantiation", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "phase_selection", uparams.ParamUInt, "phase selection heuristic: 0 - always false, 1 - always true, 2 - phase caching, 3 - phase caching conservative, 4 - phase caching conservative 2, 5 - random, 6 - number of occurrences", "3", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "restart_strategy", uparams.ParamUInt, "0 - geometric, 1 - inner-outer-geometric, 2 - luby, 3 - fixed, 4 - arithmetic", "1", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "restart_factor", uparams.ParamDouble, "when using geometric (or inner-outer-geometric) progression of restarts, it specifies the constant used to multiply the currect restart threshold", "1.1", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "case_split", uparams.ParamUInt, "0 - case split based on variable activity, 1 - similar to 0, but delay case splits created during the search, 2 - similar to 0, but cache the relevancy, 3 - case split based on relevancy (structural splitting), 4 - case split on relevancy and activity, 5 - case split on relevancy and current goal, 6 - activity-based case split with theory-aware branching activity", "1", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "delay_units", uparams.ParamBool, "if true then z3 will not restart when a unit clause is learned", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "delay_units_threshold", uparams.ParamUInt, "maximum number of learned unit clauses before restarting, ignored if delay_units is false", "32", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pull_nested_quantifiers", uparams.ParamBool, "pull nested quantifiers", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "refine_inj_axioms", uparams.ParamBool, "refine injectivity axioms", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "timeout", uparams.ParamUInt, "timeout (in milliseconds) (UINT_MAX and 0 mean no timeout)", "4294967295", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "rlimit", uparams.ParamUInt, "resource limit (0 means no limit)", "0", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "max_conflicts", uparams.ParamUInt, "maximum number of conflicts before giving up.", "4294967295", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "mbqi", uparams.ParamBool, "model based quantifier instantiation (MBQI)", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "mbqi.max_cexs", uparams.ParamUInt, "initial maximal number of counterexamples used in MBQI, each counterexample generates a quantifier instantiation", "1", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "mbqi.max_cexs_incr", uparams.ParamUInt, "increment for MBQI_MAX_CEXS, the increment is performed after each round of MBQI", "0", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "mbqi.max_iterations", uparams.ParamUInt, "maximum number of rounds of MBQI", "1000", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "mbqi.trace", uparams.ParamBool, "generate tracing messages for Model Based Quantifier Instantiation (MBQI). It will display a message before every round of MBQI, and the quantifiers that were not satisfied", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "mbqi.force_template", uparams.ParamUInt, "some quantifiers can be used as templates for building interpretations for functions. Z3 uses heuristics to decide whether a quantifier will be used as a template or not. Quantifiers with weight >= mbqi.force_template are forced to be used as a template", "10", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "mbqi.id", uparams.ParamString, "Only use model-based instantiation for quantifiers with id's beginning with string", "", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "qi.profile", uparams.ParamBool, "profile quantifier instantiation", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "qi.profile_freq", uparams.ParamUInt, "how frequent results are reported by qi.profile", "4294967295", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "qi.max_instances", uparams.ParamUInt, "maximum number of quantifier instantiations", "4294967295", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "qi.eager_threshold", uparams.ParamDouble, "threshold for eager quantifier instantiation", "10.0", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "qi.lazy_threshold", uparams.ParamDouble, "threshold for lazy quantifier instantiation", "20.0", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "qi.cost", uparams.ParamString, "expression specifying what is the cost of a given quantifier instantiation", "(+ weight generation)", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "qi.max_multi_patterns", uparams.ParamUInt, "specify the number of extra multi patterns", "0", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv.reflect", uparams.ParamBool, "create enode for every bit-vector term", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv.enable_int2bv", uparams.ParamBool, "enable support for int2bv and bv2int operators", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.random_initial_value", uparams.ParamBool, "use random initial values in the simplex-based procedure for linear arithmetic", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.solver", uparams.ParamUInt, "arithmetic solver: 0 - no solver, 1 - bellman-ford based solver (diff. logic only), 2 - simplex based solver, 3 - floyd-warshall based solver (diff. logic only) and no theory combination", "2", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.nl", uparams.ParamBool, "(incomplete) nonlinear arithmetic support based on Groebner basis and interval propagation", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.nl.gb", uparams.ParamBool, "groebner Basis computation, this option is ignored when arith.nl=false", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.nl.branching", uparams.ParamBool, "branching on integer variables in non linear clusters", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.nl.rounds", uparams.ParamUInt, "threshold for number of (nested) final checks for non linear arithmetic", "1024", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.euclidean_solver", uparams.ParamBool, "eucliean solver for linear integer arithmetic", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.propagate_eqs", uparams.ParamBool, "propagate (cheap) equalities", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.propagation_mode", uparams.ParamUInt, "0 - no propagation, 1 - propagate existing literals, 2 - refine bounds", "2", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.reflect", uparams.ParamBool, "reflect arithmetical operators to the congruence closure", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.branch_cut_ratio", uparams.ParamUInt, "branch/cut ratio for linear integer arithmetic", "2", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.int_eq_branch", uparams.ParamBool, "branching using derived integer equations", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.ignore_int", uparams.ParamBool, "treat integer variables as real", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.dump_lemmas", uparams.ParamBool, "dump arithmetic theory lemmas to files", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith.greatest_error_pivot", uparams.ParamBool, "Pivoting strategy", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pb.conflict_frequency", uparams.ParamUInt, "conflict frequency for Pseudo-Boolean theory", "1000", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pb.learn_complements", uparams.ParamBool, "learn complement literals for Pseudo-Boolean theory", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pb.enable_compilation", uparams.ParamBool, "enable compilation into sorting circuits for Pseudo-Boolean", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pb.enable_simplex", uparams.ParamBool, "enable simplex to check rational feasibility", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "array.weak", uparams.ParamBool, "weak array theory", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "array.extensional", uparams.ParamBool, "extensional array theory", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "dack", uparams.ParamUInt, "0 - disable dynamic ackermannization, 1 - expand Leibniz's axiom if a congruence is the root of a conflict, 2 - expand Leibniz's axiom if a congruence is used during conflict resolution", "1", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "dack.eq", uparams.ParamBool, "enable dynamic ackermannization for transtivity of equalities", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "dack.factor", uparams.ParamDouble, "number of instance per conflict", "0.1", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "dack.gc", uparams.ParamUInt, "Dynamic ackermannization garbage collection frequency (per conflict)", "2000", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "dack.gc_inv_decay", uparams.ParamDouble, "Dynamic ackermannization garbage collection decay", "0.8", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "dack.threshold", uparams.ParamUInt, " number of times the congruence rule must be used before Leibniz's axiom is expanded", "10", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "theory_case_split", uparams.ParamBool, "Allow the context to use heuristics involving theory case splits, which are a set of literals of which exactly one can be assigned True. If this option is false, the context will generate extra axioms to enforce this instead.", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "string_solver", uparams.ParamSymbol, "solver for string/sequence theories. options are: 'z3str3' (specialized string solver), 'seq' (sequence solver), 'auto' (use static features to choose best solver)", "seq", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "core.validate", uparams.ParamBool, "validate unsat core produced by SMT context", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.strong_arrangements", uparams.ParamBool, "assert equivalences instead of implications when generating string arrangement axioms", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.aggressive_length_testing", uparams.ParamBool, "prioritize testing concrete length values over generating more options", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.aggressive_value_testing", uparams.ParamBool, "prioritize testing concrete string constant values over generating more options", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.aggressive_unroll_testing", uparams.ParamBool, "prioritize testing concrete regex unroll counts over generating more options", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.fast_length_tester_cache", uparams.ParamBool, "cache length tester constants instead of regenerating them", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.fast_value_tester_cache", uparams.ParamBool, "cache value tester constants instead of regenerating them", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.string_constant_cache", uparams.ParamBool, "cache all generated string constants generated from anywhere in theory_str", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.use_binary_search", uparams.ParamBool, "use a binary search heuristic for finding concrete length values for free variables in theory_str (set to False to use linear search)", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.binary_search_start", uparams.ParamUInt, "initial upper bound for theory_str binary search", "64", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "theory_aware_branching", uparams.ParamBool, "Allow the context to use extra information from theory solvers regarding literal branching prioritization.", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.finite_overlap_models", uparams.ParamBool, "attempt a finite model search for overlapping variables instead of completely giving up on the arrangement", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.overlap_priority", uparams.ParamDouble, "theory-aware priority for overlapping variable cases; use smt.theory_aware_branching=true", "-0.1", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.regex_automata", uparams.ParamBool, "use automata-based reasoning for regular expressions (Z3str3 only)", "true", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.regex_automata_difficulty_threshold", uparams.ParamUInt, "difficulty threshold for regex automata heuristics", "1000", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.regex_automata_intersection_difficulty_threshold", uparams.ParamUInt, "difficulty threshold for regex intersection heuristics", "1000", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.regex_automata_failed_automaton_threshold", uparams.ParamUInt, "number of failed automaton construction attempts after which a full automaton is automatically built", "10", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.regex_automata_failed_intersection_threshold", uparams.ParamUInt, "number of failed automaton intersection attempts after which intersection is always computed", "10", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "str.regex_automata_length_attempt_threshold", uparams.ParamUInt, "number of length/path constraint attempts before checking unsatisfiability of regex terms", "10", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "core.minimize", uparams.ParamBool, "minimize unsat core produced by SMT context", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "core.extend_patterns", uparams.ParamBool, "extend unsat core with literals that trigger (potential) quantifier instances", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "core.extend_patterns.max_distance", uparams.ParamUInt, "limits the distance of a pattern-extended unsat core", "4294967295", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "core.extend_nonlocal_patterns", uparams.ParamBool, "extend unsat cores with literals that have quantifiers with patterns that contain symbols which are not in the quantifier's body", "false", "smt"); e != nil {
		return e
	}
	if e := catalogInsert(d, "lemma_gc_strategy", uparams.ParamUInt, "lemma garbage collection strategy: 0 - fixed, 1 - geometric, 2 - at restart, 3 - none", "0", "smt"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectContextParams", catalogCollectContextParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectContextSolverParams", catalogCollectContextSolverParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectPatternInferenceParamsHelper", catalogCollectPatternInferenceParamsHelper)
	uparams.RegisterGlobalParamsCollector("catalogCollectInterpParams", catalogCollectInterpParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectSmtParamsHelper", catalogCollectSmtParamsHelper)
}
