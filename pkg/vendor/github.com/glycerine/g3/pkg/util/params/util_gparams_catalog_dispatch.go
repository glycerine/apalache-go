package params

var catalogDescriptionLiterals = [...]struct{ module, text string }{
	{"ackermannization", "solving UF via ackermannization"},
	{"nnf", "negation normal form"},
	{"pi", "pattern inference (heuristics) for universal formulas (without annotation)"},
	{"pp", "pretty printer"},
	{"rewriter", "new formula simplification module used in the tactic framework, and new solvers"},
	{"interp", "interpolation parameters"},
	{"algebraic", "real algebraic number package"},
	{"rcf", "real closed fields"},
	{"fixedpoint", "fixedpoint parameters"},
	{"nlsat", "nonlinear solver"},
	{"opt", "optimization parameters"},
	{"sat", "propositional SAT solver"},
	{"smt", "smt solver based on lazy smt"},
	{"combined_solver", "combines two solvers: non-incremental (solver1) and incremental (solver2)"},
	{"sls", "Experimental Stochastic Local Search Solver (for QFBV only)."},
	{"ackermannization", "tactics based on solving UF-theories via ackermannization (see also ackr module)"},
}

func (c *GlobalParamsCatalog) dispatch(r GlobalParamsRegistrar) error {
	if e := c.global(r, catalogCollector("catalogCollectContextParams")); e != nil {
		return e
	}
	if e := c.global(r, catalogCollectEnvParams); e != nil {
		return e
	}
	if e := c.module(r, 40, "ackermannization", catalogCollector("catalogCollectAckermannizationParams")); e != nil {
		return e
	}
	if e := c.module(r, 41, "rewriter", catalogCollector("catalogCollectAckermannizeBvTacticParams")); e != nil {
		return e
	}
	if e := c.module(r, 42, "rewriter", catalogCollector("catalogCollectFpa2bvRewriterParams")); e != nil {
		return e
	}
	if e := c.module(r, 43, "nnf", catalogCollector("catalogCollectNnf")); e != nil {
		return e
	}
	if e := c.module(r, 44, "nnf", catalogCollector("catalogCollectNnfParams")); e != nil {
		return e
	}
	if e := c.module(r, 45, "pi", catalogCollector("catalogCollectPatternInferenceParamsHelper")); e != nil {
		return e
	}
	if e := c.module(r, 46, "pp", catalogCollector("catalogCollectPpParams")); e != nil {
		return e
	}
	if e := c.module(r, 47, "rewriter", catalogCollector("catalogCollectArithRewriterParams")); e != nil {
		return e
	}
	if e := c.module(r, 48, "rewriter", catalogCollector("catalogCollectArrayRewriterParams")); e != nil {
		return e
	}
	if e := c.module(r, 49, "rewriter", catalogCollector("catalogCollectBoolRewriterParams")); e != nil {
		return e
	}
	if e := c.module(r, 50, "rewriter", catalogCollector("catalogCollectBvRewriterParams")); e != nil {
		return e
	}
	if e := c.module(r, 51, "rewriter", catalogCollector("catalogCollectFpaRewriterParams")); e != nil {
		return e
	}
	if e := c.module(r, 52, "rewriter", catalogCollector("catalogCollectPolyRewriterParams")); e != nil {
		return e
	}
	if e := c.module(r, 53, "rewriter", catalogCollector("catalogCollectRewriterParams")); e != nil {
		return e
	}
	if e := c.module(r, 54, "interp", catalogCollector("catalogCollectInterpParams")); e != nil {
		return e
	}
	if e := c.module(r, 55, "algebraic", catalogCollector("catalogCollectAlgebraicParams")); e != nil {
		return e
	}
	if e := c.module(r, 56, "rcf", catalogCollector("catalogCollectRcfParams")); e != nil {
		return e
	}
	if e := c.module(r, 57, "model_evaluator", catalogCollector("catalogCollectModelEvaluatorParams")); e != nil {
		return e
	}
	if e := c.module(r, 58, "model", catalogCollector("catalogCollectModelParams")); e != nil {
		return e
	}
	if e := c.module(r, 59, "fixedpoint", catalogCollector("catalogCollectFixedpointParams")); e != nil {
		return e
	}
	if e := c.module(r, 60, "nlsat", catalogCollector("catalogCollectNlsatParams")); e != nil {
		return e
	}
	if e := c.module(r, 61, "opt", catalogCollector("catalogCollectOptParams")); e != nil {
		return e
	}
	if e := c.module(r, 62, "parser", catalogCollector("catalogCollectParserParams")); e != nil {
		return e
	}
	if e := c.module(r, 63, "sat", catalogCollector("catalogCollectSatAsymmBranchParams")); e != nil {
		return e
	}
	if e := c.module(r, 64, "sat", catalogCollector("CollectSATConfigParamDescrs")); e != nil {
		return e
	}
	if e := c.module(r, 65, "sat", catalogCollector("catalogCollectSatSccParams")); e != nil {
		return e
	}
	if e := c.module(r, 66, "sat", catalogCollector("catalogCollectSatSimplifierParams")); e != nil {
		return e
	}
	if e := c.module(r, 67, "smt", catalogCollector("catalogCollectSmtParamsHelper")); e != nil {
		return e
	}
	if e := c.module(r, 68, "combined_solver", catalogCollector("catalogCollectCombinedSolverParams")); e != nil {
		return e
	}
	if e := c.module(r, 69, "sls", catalogCollector("catalogCollectSlsParams")); e != nil {
		return e
	}
	if e := c.module(r, 70, "ackermannization", catalogCollector("catalogCollectQfufbvTacticParams")); e != nil {
		return e
	}
	if e := c.module(r, 71, "lp", catalogCollector("catalogCollectLpParams")); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("ackermannization", c.Region, c.descriptions[0], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("nnf", c.Region, c.descriptions[1], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("pi", c.Region, c.descriptions[2], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("pp", c.Region, c.descriptions[3], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("rewriter", c.Region, c.descriptions[4], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("interp", c.Region, c.descriptions[5], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("algebraic", c.Region, c.descriptions[6], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("rcf", c.Region, c.descriptions[7], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("fixedpoint", c.Region, c.descriptions[8], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("nlsat", c.Region, c.descriptions[9], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("opt", c.Region, c.descriptions[10], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("sat", c.Region, c.descriptions[11], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("smt", c.Region, c.descriptions[12], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("combined_solver", c.Region, c.descriptions[13], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("sls", c.Region, c.descriptions[14], 0); e != nil {
		return e
	}
	if e := r.RegisterModuleDescription("ackermannization", c.Region, c.descriptions[15], 0); e != nil {
		return e
	}
	return nil
}
