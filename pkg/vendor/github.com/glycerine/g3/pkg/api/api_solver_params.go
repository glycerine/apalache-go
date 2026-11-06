package api

// Public typed names for the retained SMT solver parameter catalog. The
// distinct key types prevent, for example, supplying a floating-point value
// to an unsigned parameter while retaining direct access to future keys by an
// explicit conversion.

type SMTBoolParameter string
type SMTUIntParameter string
type SMTDoubleParameter string
type SMTSymbolParameter string

const (
	SMTAutoConfig               SMTBoolParameter = "smt.auto_config"
	SMTMacroFinder              SMTBoolParameter = "smt.macro_finder"
	SMTQuasiMacros              SMTBoolParameter = "smt.quasi_macros"
	SMTRestrictedQuasiMacros    SMTBoolParameter = "smt.restricted_quasi_macros"
	SMTEMatching                SMTBoolParameter = "smt.ematching"
	SMTDelayUnits               SMTBoolParameter = "smt.delay_units"
	SMTPullNestedQuantifiers    SMTBoolParameter = "smt.pull_nested_quantifiers"
	SMTRefineInjectivityAxioms  SMTBoolParameter = "smt.refine_inj_axioms"
	SMTMBQI                     SMTBoolParameter = "smt.mbqi"
	SMTMBQITrace                SMTBoolParameter = "smt.mbqi.trace"
	SMTQIProfile                SMTBoolParameter = "smt.qi.profile"
	SMTBitVectorReflect         SMTBoolParameter = "smt.bv.reflect"
	SMTBitVectorEnableInt2BV    SMTBoolParameter = "smt.bv.enable_int2bv"
	SMTArithmeticRandomInitial  SMTBoolParameter = "smt.arith.random_initial_value"
	SMTArithmeticNonlinear      SMTBoolParameter = "smt.arith.nl"
	SMTArithmeticGroebner       SMTBoolParameter = "smt.arith.nl.gb"
	SMTArithmeticNLBranching    SMTBoolParameter = "smt.arith.nl.branching"
	SMTArithmeticEuclidean      SMTBoolParameter = "smt.arith.euclidean_solver"
	SMTArithmeticPropagateEqs   SMTBoolParameter = "smt.arith.propagate_eqs"
	SMTArithmeticReflect        SMTBoolParameter = "smt.arith.reflect"
	SMTArithmeticIntEqBranch    SMTBoolParameter = "smt.arith.int_eq_branch"
	SMTArithmeticIgnoreInt      SMTBoolParameter = "smt.arith.ignore_int"
	SMTArithmeticDumpLemmas     SMTBoolParameter = "smt.arith.dump_lemmas"
	SMTArithmeticGreatestPivot  SMTBoolParameter = "smt.arith.greatest_error_pivot"
	SMTPBLearnComplements       SMTBoolParameter = "smt.pb.learn_complements"
	SMTPBEnableCompilation      SMTBoolParameter = "smt.pb.enable_compilation"
	SMTPBEnableSimplex          SMTBoolParameter = "smt.pb.enable_simplex"
	SMTArrayWeak                SMTBoolParameter = "smt.array.weak"
	SMTArrayExtensional         SMTBoolParameter = "smt.array.extensional"
	SMTDynamicAckermannEquality SMTBoolParameter = "smt.dack.eq"
	SMTTheoryCaseSplit          SMTBoolParameter = "smt.theory_case_split"
	SMTCoreValidate             SMTBoolParameter = "smt.core.validate"
	SMTTheoryAwareBranching     SMTBoolParameter = "smt.theory_aware_branching"
	SMTStringStrongArrangements SMTBoolParameter = "smt.str.strong_arrangements"
	SMTStringAggressiveLength   SMTBoolParameter = "smt.str.aggressive_length_testing"
	SMTStringAggressiveValue    SMTBoolParameter = "smt.str.aggressive_value_testing"
	SMTStringAggressiveUnroll   SMTBoolParameter = "smt.str.aggressive_unroll_testing"
	SMTStringFastLengthCache    SMTBoolParameter = "smt.str.fast_length_tester_cache"
	SMTStringFastValueCache     SMTBoolParameter = "smt.str.fast_value_tester_cache"
	SMTStringConstantCache      SMTBoolParameter = "smt.str.string_constant_cache"
	SMTStringBinarySearch       SMTBoolParameter = "smt.str.use_binary_search"
	SMTStringFiniteOverlap      SMTBoolParameter = "smt.str.finite_overlap_models"
	SMTStringRegexAutomata      SMTBoolParameter = "smt.str.regex_automata"
	SMTCoreMinimize             SMTBoolParameter = "smt.core.minimize"
	SMTCoreExtendPatterns       SMTBoolParameter = "smt.core.extend_patterns"
	SMTCoreExtendNonlocal       SMTBoolParameter = "smt.core.extend_nonlocal_patterns"
)

const (
	SMTRandomSeed                  SMTUIntParameter = "smt.random_seed"
	SMTRelevancy                   SMTUIntParameter = "smt.relevancy"
	SMTPhaseSelection              SMTUIntParameter = "smt.phase_selection"
	SMTRestartStrategy             SMTUIntParameter = "smt.restart_strategy"
	SMTCaseSplit                   SMTUIntParameter = "smt.case_split"
	SMTDelayUnitsThreshold         SMTUIntParameter = "smt.delay_units_threshold"
	SMTTimeout                     SMTUIntParameter = "smt.timeout"
	SMTResourceLimit               SMTUIntParameter = "smt.rlimit"
	SMTMaxConflicts                SMTUIntParameter = "smt.max_conflicts"
	SMTMBQIMaxCounterexamples      SMTUIntParameter = "smt.mbqi.max_cexs"
	SMTMBQIMaxCounterexampleIncr   SMTUIntParameter = "smt.mbqi.max_cexs_incr"
	SMTMBQIMaxIterations           SMTUIntParameter = "smt.mbqi.max_iterations"
	SMTMBQIForceTemplate           SMTUIntParameter = "smt.mbqi.force_template"
	SMTQIProfileFrequency          SMTUIntParameter = "smt.qi.profile_freq"
	SMTQIMaxInstances              SMTUIntParameter = "smt.qi.max_instances"
	SMTQIMaxMultiPatterns          SMTUIntParameter = "smt.qi.max_multi_patterns"
	SMTArithmeticSolver            SMTUIntParameter = "smt.arith.solver"
	SMTArithmeticNLRounds          SMTUIntParameter = "smt.arith.nl.rounds"
	SMTArithmeticPropagationMode   SMTUIntParameter = "smt.arith.propagation_mode"
	SMTArithmeticBranchCutRatio    SMTUIntParameter = "smt.arith.branch_cut_ratio"
	SMTPBConflictFrequency         SMTUIntParameter = "smt.pb.conflict_frequency"
	SMTDynamicAckermannMode        SMTUIntParameter = "smt.dack"
	SMTDynamicAckermannGCFrequency SMTUIntParameter = "smt.dack.gc"
	SMTDynamicAckermannThreshold   SMTUIntParameter = "smt.dack.threshold"
	SMTStringBinarySearchStart     SMTUIntParameter = "smt.str.binary_search_start"
	SMTStringRegexDifficulty       SMTUIntParameter = "smt.str.regex_automata_difficulty_threshold"
	SMTStringRegexIntersection     SMTUIntParameter = "smt.str.regex_automata_intersection_difficulty_threshold"
	SMTStringRegexFailedAutomaton  SMTUIntParameter = "smt.str.regex_automata_failed_automaton_threshold"
	SMTStringRegexFailedIntersect  SMTUIntParameter = "smt.str.regex_automata_failed_intersection_threshold"
	SMTStringRegexLengthAttempts   SMTUIntParameter = "smt.str.regex_automata_length_attempt_threshold"
	SMTCoreExtendPatternDistance   SMTUIntParameter = "smt.core.extend_patterns.max_distance"
	SMTLemmaGCStrategy             SMTUIntParameter = "smt.lemma_gc_strategy"
)

const (
	SMTRestartFactor           SMTDoubleParameter = "smt.restart_factor"
	SMTQIEagerThreshold        SMTDoubleParameter = "smt.qi.eager_threshold"
	SMTQILazyThreshold         SMTDoubleParameter = "smt.qi.lazy_threshold"
	SMTDynamicAckermannFactor  SMTDoubleParameter = "smt.dack.factor"
	SMTDynamicAckermannGCDecay SMTDoubleParameter = "smt.dack.gc_inv_decay"
	SMTStringOverlapPriority   SMTDoubleParameter = "smt.str.overlap_priority"
)

const (
	SMTLogic        SMTSymbolParameter = "smt.logic"
	SMTMBQIID       SMTSymbolParameter = "smt.mbqi.id"
	SMTQICost       SMTSymbolParameter = "smt.qi.cost"
	SMTStringSolver SMTSymbolParameter = "smt.string_solver"
)

func (c *SMTContext) SetSMTBool(parameter SMTBoolParameter, value bool) error {
	return c.SetBoolOption(string(parameter), value)
}

func (c *SMTContext) SetSMTUInt(parameter SMTUIntParameter, value uint32) error {
	return c.SetUIntOption(string(parameter), value)
}

func (c *SMTContext) SetSMTDouble(parameter SMTDoubleParameter, value float64) error {
	return c.SetDoubleOption(string(parameter), value)
}

func (c *SMTContext) SetSMTSymbol(parameter SMTSymbolParameter, value string) error {
	return c.SetSymbolOption(string(parameter), value)
}
