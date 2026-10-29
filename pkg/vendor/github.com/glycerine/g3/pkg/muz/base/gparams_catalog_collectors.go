package base

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

// Source: src/muz/base/fixedpoint_params.pyg
func catalogCollectFixedpointParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "timeout", uparams.ParamUInt, "set timeout", "4294967295", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "engine", uparams.ParamSymbol, "Select: auto-config, datalog, duality, pdr, bmc, spacer", "auto-config", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.default_table", uparams.ParamSymbol, "default table implementation: sparse, hashtable, bitvector, interval", "sparse", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.default_relation", uparams.ParamSymbol, "default relation implementation: external_relation, pentagon", "pentagon", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.generate_explanations", uparams.ParamBool, "produce explanations for produced facts when using the datalog engine", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.use_map_names", uparams.ParamBool, "use names from map files when displaying tuples", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.magic_sets_for_queries", uparams.ParamBool, "magic set transformation will be used for queries", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.explanations_on_relation_level", uparams.ParamBool, "if true, explanations are generated as history of each relation, rather than per fact (generate_explanations must be set to true for this option to have any effect)", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.unbound_compressor", uparams.ParamBool, "auxiliary relations will be introduced to avoid unbound variables in rule heads", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.similarity_compressor", uparams.ParamBool, "rules that differ only in values of constants will be merged into a single rule", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.similarity_compressor_threshold", uparams.ParamUInt, "if similarity_compressor is on, this value determines how many similar rules there must be in order for them to be merged", "11", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.all_or_nothing_deltas", uparams.ParamBool, "compile rules so that it is enough for the delta relation in union and widening operations to determine only whether the updated relation was modified or not", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.compile_with_widening", uparams.ParamBool, "widening will be used to compile recursive rules", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.default_table_checked", uparams.ParamBool, "if true, the default table will be default_table inside a wrapper that checks that its results are the same as of default_table_checker table", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.default_table_checker", uparams.ParamSymbol, "see default_table_checked", "null", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.check_relation", uparams.ParamSymbol, "name of default relation to check. operations on the default relation will be verified using SMT solving", "null", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.initial_restart_timeout", uparams.ParamUInt, "length of saturation run before the first restart (in ms), zero means no restarts", "0", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.output_profile", uparams.ParamBool, "determines whether profile information should be output when outputting Datalog rules or instructions", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.print.tuples", uparams.ParamBool, "determines whether tuples for output predicates should be output", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.profile_timeout_milliseconds", uparams.ParamUInt, "instructions and rules that took less than the threshold will not be printed when printed the instruction/rule list", "0", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.dbg_fpr_nonempty_relation_signature", uparams.ParamBool, "if true, finite_product_relation will attempt to avoid creating inner relation with empty signature by putting in half of the table columns, if it would have been empty otherwise", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "datalog.subsumption", uparams.ParamBool, "if true, removes/filters predicates with total transitions", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "duality.full_expand", uparams.ParamBool, "Fully expand derivation trees", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "duality.no_conj", uparams.ParamBool, "No forced covering (conjectures)", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "duality.feasible_edges", uparams.ParamBool, "Don't expand definitely infeasible edges", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "duality.use_underapprox", uparams.ParamBool, "Use underapproximations", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "duality.stratified_inlining", uparams.ParamBool, "Use stratified inlining", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "duality.recursion_bound", uparams.ParamUInt, "Recursion bound for stratified inlining", "4294967295", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "duality.profile", uparams.ParamBool, "profile run time", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "duality.mbqi", uparams.ParamBool, "use model-based quantifier instantiation", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "duality.batch_expand", uparams.ParamBool, "use batch expansion", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "duality.conjecture_file", uparams.ParamString, "save conjectures to file", "", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.bfs_model_search", uparams.ParamBool, "use BFS strategy for expanding model search", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.farkas", uparams.ParamBool, "use lemma generator based on Farkas (for linear real arithmetic)", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "generate_proof_trace", uparams.ParamBool, "trace for 'sat' answer as proof object", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.flexible_trace", uparams.ParamBool, "allow PDR generate long counter-examples by extending candidate trace within search area", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.flexible_trace_depth", uparams.ParamUInt, "Controls the depth (below the current level) at which flexible trace can be applied", "4294967295", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.use_model_generalizer", uparams.ParamBool, "use model for backwards propagation (instead of symbolic simulation)", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.validate_result", uparams.ParamBool, "validate result (by proof checking or model checking)", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.simplify_formulas_pre", uparams.ParamBool, "simplify derived formulas before inductive propagation", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.simplify_formulas_post", uparams.ParamBool, "simplify derived formulas after inductive propagation", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.use_multicore_generalizer", uparams.ParamBool, "extract multiple cores for blocking states", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.use_inductive_generalizer", uparams.ParamBool, "generalize lemmas using induction strengthening", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.use_arith_inductive_generalizer", uparams.ParamBool, "generalize lemmas using arithmetic heuristics for induction strengthening", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.use_convex_closure_generalizer", uparams.ParamBool, "generalize using convex closures of lemmas", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.use_convex_interior_generalizer", uparams.ParamBool, "generalize using convex interiors of lemmas", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.cache_mode", uparams.ParamUInt, "use no (0), symbolic (1) or explicit cache (2) for model search", "0", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.inductive_reachability_check", uparams.ParamBool, "assume negation of the cube on the previous level when checking for reachability (not only during cube weakening)", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.max_num_contexts", uparams.ParamUInt, "maximal number of contexts to create", "500", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.try_minimize_core", uparams.ParamBool, "try to reduce core size (before inductive minimization)", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pdr.utvpi", uparams.ParamBool, "Enable UTVPI strategy", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "print_fixedpoint_extensions", uparams.ParamBool, "use SMT-LIB2 fixedpoint extensions, instead of pure SMT2, when printing rules", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "print_low_level_smt2", uparams.ParamBool, "use (faster) low-level SMT2 printer (the printer is scalable but the result may not be as readable)", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "print_with_variable_declarations", uparams.ParamBool, "use variable declarations when displaying rules (instead of attempting to use original names)", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "print_answer", uparams.ParamBool, "print answer instance(s) to query", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "print_certificate", uparams.ParamBool, "print certificate for reachability or non-reachability", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "print_boogie_certificate", uparams.ParamBool, "print certificate for reachability or non-reachability using a format understood by Boogie", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "print_statistics", uparams.ParamBool, "print statistics", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "print_aig", uparams.ParamSymbol, "Dump clauses in AIG text format (AAG) to the given file name", "", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "tab.selection", uparams.ParamSymbol, "selection method for tabular strategy: weight (default), first, var-use", "weight", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.bit_blast", uparams.ParamBool, "bit-blast bit-vectors", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.magic", uparams.ParamBool, "perform symbolic magic set transformation", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.scale", uparams.ParamBool, "add scaling variable to linear real arithmetic clauses", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.inline_linear", uparams.ParamBool, "try linear inlining method", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.inline_eager", uparams.ParamBool, "try eager inlining of rules", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.inline_linear_branch", uparams.ParamBool, "try linear inlining method with potential expansion", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.compress_unbound", uparams.ParamBool, "compress tails with unbound variables", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.fix_unbound_vars", uparams.ParamBool, "fix unbound variables in tail", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.unfold_rules", uparams.ParamUInt, "unfold rules statically using iterative squarring", "0", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.slice", uparams.ParamBool, "simplify clause set using slicing", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.karr", uparams.ParamBool, "Add linear invariants to clauses using Karr's method", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.use_eqclass", uparams.ParamBool, "Generalizes equalities to equivalence classes", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.transform_arrays", uparams.ParamBool, "Rewrites arrays equalities and applies select over store", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.instantiate_arrays", uparams.ParamBool, "Transforms P(a) into P(i, a[i] a)", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.instantiate_arrays.enforce", uparams.ParamBool, "Transforms P(a) into P(i, a[i]), discards a from predicate", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.instantiate_arrays.nb_quantifier", uparams.ParamUInt, "Gives the number of quantifiers per array", "1", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.instantiate_arrays.slice_technique", uparams.ParamSymbol, "<no-slicing>=> GetId(i) = i, <smash> => GetId(i) = true", "no-slicing", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.quantify_arrays", uparams.ParamBool, "create quantified Horn clauses from clauses with arrays", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.instantiate_quantifiers", uparams.ParamBool, "instantiate quantified Horn clauses using E-matching heuristic", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.coalesce_rules", uparams.ParamBool, "coalesce rules", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.tail_simplifier_pve", uparams.ParamBool, "propagate_variable_equivalences", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.subsumption_checker", uparams.ParamBool, "Enable subsumption checker (no support for model conversion)", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.coi", uparams.ParamBool, "use cone of influence simplification", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "duality.enable_restarts", uparams.ParamBool, "DUALITY: enable restarts", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.order_children", uparams.ParamUInt, "SPACER: order of enqueuing children in non-linear rules : 0 (original), 1 (reverse)", "0", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.eager_reach_check", uparams.ParamBool, "SPACER: eagerly check if a query is reachable using reachability facts of predecessors", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.use_lemma_as_cti", uparams.ParamBool, "SPACER: use a lemma instead of a CTI in flexible_trace", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.reset_obligation_queue", uparams.ParamBool, "SPACER: reset obligation queue when entering a new level", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.init_reach_facts", uparams.ParamBool, "SPACER: initialize reachability facts with false", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.use_array_eq_generalizer", uparams.ParamBool, "SPACER: attempt to generalize lemmas with array equalities", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.use_derivations", uparams.ParamBool, "SPACER: using derivation mechanism to cache intermediate results for non-linear rules", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.array_blast", uparams.ParamBool, "try to eliminate local array terms using Ackermannization -- some array terms may remain", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "xform.array_blast_full", uparams.ParamBool, "eliminate all local array variables by QE", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.skip_propagate", uparams.ParamBool, "Skip propagate/pushing phase. Turns PDR into a BMC that returns either reachable or unknown", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.max_level", uparams.ParamUInt, "Maximum level to explore", "4294967295", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.elim_aux", uparams.ParamBool, "Eliminate auxiliary variables in reachability facts", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.reach_as_init", uparams.ParamBool, "Extend initial rules with computed reachability facts", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.blast_term_ite", uparams.ParamBool, "Expand non-Boolean ite-terms", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.nondet_tie_break", uparams.ParamBool, "Break ties in obligation queue non-deterministically", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.reach_dnf", uparams.ParamBool, "Restrict reachability facts to DNF", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bmc.linear_unrolling_depth", uparams.ParamUInt, "Maximal level to explore", "4294967295", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.split_farkas_literals", uparams.ParamBool, "Split Farkas literals", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.native_mbp", uparams.ParamBool, "Use native mbp of Z3", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.eq_prop", uparams.ParamBool, "Enable equality and bound propagation in arithmetic", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.weak_abs", uparams.ParamBool, "Weak abstraction", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.restarts", uparams.ParamBool, "Enable reseting obligation queue", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.restart_initial_threshold", uparams.ParamUInt, "Intial threshold for restarts", "10", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.random_seed", uparams.ParamUInt, "Random seed to be used by SMT solver", "0", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.ground_cti", uparams.ParamBool, "Require CTI to be ground", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.vs.dump_benchmarks", uparams.ParamBool, "dump benchmarks in virtual solver", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.vs.dump_min_time", uparams.ParamDouble, "min time to dump benchmark", "5.0", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.vs.recheck", uparams.ParamBool, "re-check locally during benchmark dumping", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.mbqi", uparams.ParamBool, "use model-based quantifier instantiation", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.keep_proxy", uparams.ParamBool, "keep proxy variables (internal parameter)", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.instantiate", uparams.ParamBool, "instantiate quantified lemmas", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.qlemmas", uparams.ParamBool, "allow quantified lemmas in frames", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.new_unsat_core", uparams.ParamBool, "use the new implementation of unsat-core-generation", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.minimize_unsat_core", uparams.ParamBool, "compute unsat-core by min-cut", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.farkas_optimized", uparams.ParamBool, "use the optimized farkas plugin, which performs gaussian elimination", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.farkas_a_const", uparams.ParamBool, "if the unoptimized farkas plugin is used, use the constants from A while constructing unsat_cores", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.lemma_sanity_check", uparams.ParamBool, "check during generalization whether lemma is actually correct", "false", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.reuse_pobs", uparams.ParamBool, "reuse POBs", "true", "fixedpoint"); e != nil {
		return e
	}
	if e := catalogInsert(d, "spacer.simplify_pob", uparams.ParamBool, "simplify POBs by removing redundant constraints", "false", "fixedpoint"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectFixedpointParams", catalogCollectFixedpointParams)
}
