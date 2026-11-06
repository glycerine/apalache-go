package spacer

// Source: retained construction, init_rules, solve_core, and propagation in
// src/muz/spacer/spacer_context.h and spacer_context.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

var ErrContextOwner = errors.New("muz/spacer: invalid or closed context")

type Context struct {
	manager                 *ast.ASTManager
	owner                   *Manager
	config                  PropSolverConfig
	transformers            map[ast.ASTHandle]*PredicateTransformer
	order                   []ast.ASTHandle
	query                   *PredicateTransformer
	queryRef                ast.ASTRef
	queryDecl               ast.ASTHandle
	queue                   ProofObligationQueue
	lastResult              lbool.LBool
	inductive               uint32
	cexDepth                uint32
	cexDepthSet             bool
	stats                   contextStatistics
	generalizerStats        boolGeneralizerStatistics
	generalizersInitialized bool
	timers                  contextTiming
	skipPropagate           bool
	resetQueue              bool
	flexibleTrace           bool
	flexibleDepth           uint32
	reusePOBs               bool
	simplifyPOBs            bool
	blastTermITE            bool
	initReachFacts          bool
	reachAsInitial          bool
	useDerivations          bool
	reverseChildren         bool
	eliminateAux            bool
	reachDNF                bool
	useRestarts             bool
	restartInitial          uint32
	instantiate             bool
	quantifiedLemmas        bool
	groundCTI               bool
	weakAbstraction         bool
	validateResult          bool
	printStatistics         bool
	simplifyPre             bool
	simplifyPost            bool
	initialized             bool
	resetting               bool
	solved                  bool
	bounded                 bool
	closed                  bool
}

func NewContext(manager *ast.ASTManager, params *uparams.ParamsRef, maximumContexts uint32, config smtcore.SMTSolverConfig) (result *Context, resultErr error) {
	if manager == nil {
		return nil, ErrContextOwner
	}
	owner, err := NewManager(manager, params, maximumContexts, config)
	if err != nil {
		return nil, err
	}
	propConfig, err := spacerPropSolverConfig(params, config)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	parameters, err := newFixedpointParamView(params, config.Params, config.Globals)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	skipPropagate, err := parameters.Bool("spacer.skip_propagate", false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	resetQueue, err := parameters.Bool("spacer.reset_obligation_queue", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	flexibleTrace, err := parameters.Bool("pdr.flexible_trace", false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	flexibleDepth, err := parameters.UInt("pdr.flexible_trace_depth", InfinityLevel)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	getBool := func(name string, fallback bool) (bool, error) {
		return parameters.Bool(name, fallback)
	}
	reusePOBs, err := getBool("spacer.reuse_pobs", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	simplifyPOBs, err := getBool("spacer.simplify_pob", false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	blastTermITE, err := getBool("spacer.blast_term_ite", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	initReachFacts, err := getBool("spacer.init_reach_facts", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	reachAsInitial, err := getBool("spacer.reach_as_init", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	useDerivations, err := getBool("spacer.use_derivations", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	orderChildren, err := parameters.UInt("spacer.order_children", 0)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	eliminateAux, err := getBool("spacer.elim_aux", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	reachDNF, err := getBool("spacer.reach_dnf", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	useRestarts, err := getBool("spacer.restarts", false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	restartInitial, err := parameters.UInt("spacer.restart_initial_threshold", 10)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	instantiate, err := getBool("spacer.instantiate", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	quantifiedLemmas, err := getBool("spacer.qlemmas", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	groundCTI, err := getBool("spacer.ground_cti", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	weakAbstraction, err := getBool("spacer.weak_abs", true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	validateResult, err := getBool("pdr.validate_result", false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	printStatistics, err := getBool("print_statistics", false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	simplifyPre, err := getBool("pdr.simplify_formulas_pre", false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	simplifyPost, err := getBool("pdr.simplify_formulas_post", false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return &Context{
		manager: manager, owner: owner, transformers: make(map[ast.ASTHandle]*PredicateTransformer), lastResult: lbool.Undef,
		config: propConfig, skipPropagate: skipPropagate, resetQueue: resetQueue, flexibleTrace: flexibleTrace, flexibleDepth: flexibleDepth,
		reusePOBs: reusePOBs, simplifyPOBs: simplifyPOBs, blastTermITE: blastTermITE, initReachFacts: initReachFacts, reachAsInitial: reachAsInitial,
		useDerivations: useDerivations, reverseChildren: orderChildren == 1,
		eliminateAux: eliminateAux, reachDNF: reachDNF,
		useRestarts: useRestarts, restartInitial: restartInitial,
		instantiate: instantiate, quantifiedLemmas: quantifiedLemmas,
		groundCTI:       groundCTI,
		weakAbstraction: weakAbstraction,
		validateResult:  validateResult,
		printStatistics: printStatistics,
		simplifyPre:     simplifyPre,
		simplifyPost:    simplifyPost,
	}, nil
}

func spacerPropSolverConfig(params *uparams.ParamsRef, config smtcore.SMTSolverConfig) (_ PropSolverConfig, resultErr error) {
	if config.Params == nil || config.Globals == nil || config.Globals.Context != config.Params {
		return PropSolverConfig{}, ErrContextOwner
	}
	parameters, err := newFixedpointParamView(params, config.Params, config.Globals)
	if err != nil {
		return PropSolverConfig{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	get := func(name string, fallback bool) (bool, error) {
		return parameters.Bool(name, fallback)
	}
	newCore, err := get("spacer.new_unsat_core", true)
	if err != nil {
		return PropSolverConfig{}, err
	}
	minimize, err := get("spacer.minimize_unsat_core", false)
	if err != nil {
		return PropSolverConfig{}, err
	}
	optimized, err := get("spacer.farkas_optimized", true)
	if err != nil {
		return PropSolverConfig{}, err
	}
	split, err := get("spacer.split_farkas_literals", false)
	if err != nil {
		return PropSolverConfig{}, err
	}
	farkasAConst, err := get("spacer.farkas_a_const", true)
	if err != nil {
		return PropSolverConfig{}, err
	}
	keepProxy, err := get("spacer.keep_proxy", true)
	if err != nil {
		return PropSolverConfig{}, err
	}
	useInductiveGeneralizer, err := get("pdr.use_inductive_generalizer", true)
	if err != nil {
		return PropSolverConfig{}, err
	}
	useArrayEqGeneralizer, err := get("spacer.use_array_eq_generalizer", true)
	if err != nil {
		return PropSolverConfig{}, err
	}
	useEqClassGeneralizer, err := get("spacer.use_eqclass", false)
	if err != nil {
		return PropSolverConfig{}, err
	}
	lemmaSanityCheck, err := get("spacer.lemma_sanity_check", false)
	if err != nil {
		return PropSolverConfig{}, err
	}
	useLemmaAsCTI, err := get("spacer.use_lemma_as_cti", false)
	if err != nil {
		return PropSolverConfig{}, err
	}
	mode := ItpCoreLegacyFarkas
	if newCore {
		mode = ItpCoreNewFarkas
		if optimized {
			mode = ItpCoreNewOptimized
		}
	}
	return PropSolverConfig{
		Itp: ItpSolverConfig{
			Params: config.Params, Globals: config.Globals, SplitLiterals: split,
			CoreMode: mode, FarkasAConst: farkasAConst, MinimizeUnsatCore: minimize,
		},
		KeepProxy: keepProxy, UseInductiveGeneralizer: useInductiveGeneralizer, UseArrayEqGeneralizer: useArrayEqGeneralizer, UseEqClassGeneralizer: useEqClassGeneralizer, LemmaSanityCheck: lemmaSanityCheck, UseLemmaAsCTI: useLemmaAsCTI,
	}, nil
}

func (c *Context) open() error {
	if c == nil || c.closed || c.manager == nil || c.owner == nil || c.owner.open() != nil {
		return ErrContextOwner
	}
	return nil
}

// SetBackground replaces the formula asserted by subsequently constructed
// predicate solvers. It must be called before rules are initialized.
func (c *Context) SetBackground(background ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.owner.SetBackground(background)
}

func closeSpacerRuleBuckets(buckets map[ast.ASTHandle][]*base.Rule) (result error) {
	for _, rules := range buckets {
		for _, rule := range rules {
			result = errors.Join(result, rule.Close())
		}
	}
	return result
}

func appendSpacerDeclaration(order *[]ast.ASTHandle, seen map[ast.ASTHandle]bool, declaration ast.ASTHandle) {
	if !seen[declaration] {
		seen[declaration] = true
		*order = append(*order, declaration)
	}
}

// Initialized reports whether a rule program has been installed.
func (c *Context) Initialized() (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	return c.initialized, nil
}

func (c *Context) Initialize(set *base.RuleSet) (resultErr error) {
	return c.InitializePredicates(set, nil)
}

// InitializePredicates constructs transformers for every registered live
// predicate, including output predicates without defining rules.
func (c *Context) InitializePredicates(set *base.RuleSet, declarations []ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	// Covers may create transformers before the first rule installation.
	// Source update_rules inherits those properties into initialized frames.
	return c.initializePredicates(set, declarations, len(c.transformers) != 0)
}

// UpdateRules follows context::update_rules. The caller must reset instead
// when new rules invalidate existing properties, as dl_interface::check_reset
// does before calling the source method.
func (c *Context) UpdateRules(set *base.RuleSet) error {
	return c.UpdateRulesPredicates(set, nil)
}

// UpdateRulesPredicates also keeps registered predicates without rules, as
// InitializePredicates does for the fixedpoint engine's predicate registry.
func (c *Context) UpdateRulesPredicates(set *base.RuleSet, declarations []ast.ASTHandle) error {
	return c.initializePredicates(set, declarations, true)
}

func (c *Context) initializePredicates(set *base.RuleSet, declarations []ast.ASTHandle, updating bool) (resultErr error) {
	if err := c.open(); err != nil {
		return err
	}
	if (!updating && c.initialized) || set == nil {
		return ast.ErrASTDomain
	}
	count, err := set.Len()
	if err != nil {
		return err
	}
	// Source update_rules recreates generalizers before init_rules, even
	// when subsequent rule installation fails. Context.Reset alone does not.
	c.generalizersInitialized = true
	c.generalizerStats = boolGeneralizerStatistics{}
	watch, err := stopwatch.NewScopedWatch(&c.timers.initRules, false)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, watch.Close()) }()
	var order []ast.ASTHandle
	seen := make(map[ast.ASTHandle]bool)
	buckets := make(map[ast.ASTHandle][]*base.Rule)
	defer func() { resultErr = errors.Join(resultErr, closeSpacerRuleBuckets(buckets)) }()
	for _, declaration := range declarations {
		appendSpacerDeclaration(&order, seen, declaration)
	}
	outputs, err := set.OutputPredicates()
	if err != nil {
		return err
	}
	for _, declaration := range outputs {
		appendSpacerDeclaration(&order, seen, declaration)
	}
	for index := 0; index < count; index++ {
		rule, err := set.Rule(index)
		if err != nil {
			return err
		}
		head, err := rule.HeadDeclaration()
		if err != nil {
			return errors.Join(err, rule.Close())
		}
		appendSpacerDeclaration(&order, seen, head)
		buckets[head] = append(buckets[head], rule)
		tails, err := rule.UninterpretedTailSize()
		if err != nil {
			return err
		}
		for tail := 0; tail < tails; tail++ {
			declaration, err := rule.TailDeclaration(tail)
			if err != nil {
				return err
			}
			appendSpacerDeclaration(&order, seen, declaration)
		}
	}
	created := make(map[ast.ASTHandle]*PredicateTransformer, len(order))
	committed := false
	defer func() {
		if committed {
			return
		}
		for index := len(order) - 1; index >= 0; index-- {
			if transformer := created[order[index]]; transformer != nil {
				resultErr = errors.Join(resultErr, transformer.Close())
			}
		}
	}()
	for _, declaration := range order {
		transformer, err := NewRulePredicateTransformer(c.owner, declaration, c.config)
		if err != nil {
			return err
		}
		transformer.context = c
		transformer.contextStats = &c.stats
		transformer.contextTimers = &c.timers
		transformer.generalizerStats = &c.generalizerStats
		transformer.sharedGeneralizerStats = true
		created[declaration] = transformer
		if err = transformer.SetProofObligationNormalization(c.simplifyPOBs, c.config.UseEqClassGeneralizer); err != nil {
			return err
		}
		if err = transformer.SetProofObligationReuse(c.reusePOBs); err != nil {
			return err
		}
		if err = transformer.SetBlastTermITE(c.blastTermITE); err != nil {
			return err
		}
		if err = transformer.SetInitializeReachFacts(c.initReachFacts); err != nil {
			return err
		}
		if err = transformer.SetReachAsInitial(c.reachAsInitial); err != nil {
			return err
		}
		if err = transformer.SetDerivationPolicy(c.useDerivations, c.reverseChildren); err != nil {
			return err
		}
		if err = transformer.SetReachFactPolicy(c.eliminateAux, c.reachDNF); err != nil {
			return err
		}
		if err = transformer.SetQuantifiedLemmaPolicy(c.instantiate, c.quantifiedLemmas); err != nil {
			return err
		}
		if err = transformer.SetGroundCTI(c.groundCTI); err != nil {
			return err
		}
		if err = transformer.SetWeakAbstraction(c.weakAbstraction); err != nil {
			return err
		}
	}
	for _, declaration := range order {
		if err = created[declaration].InitializeRules(created, buckets[declaration]); err != nil {
			return err
		}
	}
	for _, declaration := range order {
		if err = created[declaration].InitReachFacts(); err != nil {
			return err
		}
	}
	if err = watch.Close(); err != nil {
		return err
	}
	if updating {
		// Source inherits only predicates present in both rule sets, after
		// initializing the new reach facts and before destroying old search state.
		for _, declaration := range order {
			if previous := c.transformers[declaration]; previous != nil {
				if err = created[declaration].frames.inheritFrom(c.owner, previous.frames); err != nil {
					return err
				}
			}
		}
		if err = c.Reset(); err != nil {
			return err
		}
	}
	c.transformers = created
	c.order = order
	c.initialized = true
	committed = true
	return nil
}

func (c *Context) SetQuery(declaration ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if !c.initialized || c.solved {
		return ast.ErrASTDomain
	}
	transformer := c.transformers[declaration]
	if transformer == nil {
		return ast.ErrASTDomain
	}
	if c.queryRef == (ast.ASTRef{}) {
		ref, err := c.manager.NewRef(declaration)
		if err != nil {
			return err
		}
		c.queryRef = ref
	} else if err := c.manager.SetRef(c.queryRef, declaration); err != nil {
		return err
	}
	c.queryDecl = declaration
	c.query = transformer
	return nil
}

func (c *Context) Transformer(declaration ast.ASTHandle) (*PredicateTransformer, bool, error) {
	if err := c.open(); err != nil {
		return nil, false, err
	}
	transformer, found := c.transformers[declaration]
	return transformer, found, nil
}

func (c *Context) NumLevels(declaration ast.ASTHandle) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	transformer := c.transformers[declaration]
	if transformer == nil {
		return 0, c.writeMissingPredicate(declaration)
	}
	return transformer.NumLevels()
}

// CoverDelta returns one predicate cover frame as an owning AST reference.
// Level -1 selects the invariant frame.
func (c *Context) CoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	transformer := c.transformers[declaration]
	if transformer == nil {
		truth, err := c.manager.MkTrue()
		if err != nil {
			return ast.ASTRef{}, err
		}
		return c.manager.NewRef(truth)
	}
	return transformer.CoverDelta(level)
}

// AddCover adds a bound-variable property to one predicate frame.
func (c *Context) AddCover(level int, declaration, property ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.resetting {
		return ErrContextOwner
	}
	transformer := c.transformers[declaration]
	if transformer == nil {
		var err error
		transformer, err = NewRulePredicateTransformer(c.owner, declaration, c.config)
		if err != nil {
			return err
		}
		transformer.context = c
		transformer.contextStats = &c.stats
		transformer.contextTimers = &c.timers
		transformer.generalizerStats = &c.generalizerStats
		transformer.sharedGeneralizerStats = true
		c.transformers[declaration] = transformer
		c.order = append(c.order, declaration)
		if err = c.writeMissingPredicate(declaration); err != nil {
			return err
		}
	}
	return transformer.AddCover(level, property)
}

// AddInvariant installs a property in the predicate's invariant frame.
func (c *Context) AddInvariant(declaration, property ast.ASTHandle) error {
	return c.AddCover(-1, declaration, property)
}

// Reachable returns the accumulated concrete reach summary for a predicate.
func (c *Context) Reachable(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	transformer := c.transformers[declaration]
	if transformer == nil {
		falsity, err := c.manager.MkFalse()
		if err != nil {
			return ast.ASTRef{}, err
		}
		return c.manager.NewRef(falsity)
	}
	return transformer.Reachable()
}

func (c *Context) propagate(minimum, maximum uint32) (_ bool, resultErr error) {
	watch, err := stopwatch.NewScopedWatch(&c.timers.propagate, false)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, watch.Close()) }()
	if minimum == InfinityLevel {
		return false, nil
	}
	if c.simplifyPre {
		if err := c.simplifyFormulas(); err != nil {
			return false, err
		}
	}
	for level := minimum; ; level++ {
		if err := spacerCheckpoint(c.manager); err != nil {
			return false, err
		}
		all := true
		for _, declaration := range c.order {
			if err := spacerCheckpoint(c.manager); err != nil {
				return false, err
			}
			propagated, err := c.transformers[declaration].PropagateToNextLevel(level)
			if err != nil {
				return false, err
			}
			all = all && propagated
		}
		if all {
			for _, declaration := range c.order {
				if err := spacerCheckpoint(c.manager); err != nil {
					return false, err
				}
				if err := c.transformers[declaration].PropagateToInfinity(level); err != nil {
					return false, err
				}
			}
			if level <= maximum {
				c.inductive = level
				return true, nil
			}
			break
		}
		if level == InfinityLevel {
			break
		}
	}
	if c.simplifyPost {
		if err := c.simplifyFormulas(); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (c *Context) simplifyFormulas() error {
	for _, declaration := range c.order {
		if err := c.transformers[declaration].frames.Simplify(c.owner); err != nil {
			return err
		}
	}
	return nil
}

func (c *Context) lemmaAdditions() (uint64, error) {
	return uint64(c.stats.lemmas), nil
}

// Solve runs bounded Spacer search starting at fromLevel. maxLevels is the
// number of search levels to attempt before returning undef.
func (c *Context) Solve(fromLevel, maxLevels uint32) (_ lbool.LBool, resultErr error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	if c.resetting {
		return lbool.Undef, ErrContextOwner
	}
	if !c.initialized || c.queryDecl.IsNull() || c.solved {
		return lbool.Undef, ast.ErrASTDomain
	}
	c.bounded = false
	watch, err := stopwatch.NewScopedWatch(&c.timers.solve, false)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		if resultErr == nil {
			resultErr = c.printSolveStatistics()
		}
	}()
	defer func() { resultErr = errors.Join(resultErr, watch.Close()) }()
	// update_rules clears the old transformer pointer but preserves the
	// selected declaration. Source solve_core resolves it in the new map.
	c.query = c.transformers[c.queryDecl]
	if c.query == nil {
		return c.finishEarlyUnreachable(fromLevel)
	}
	// The Go context also constructs transformers for registered/output
	// predicates that the source init_rules traversal never encounters. Treat
	// such a metadata-only query like the source's absent transformer, while a
	// ruleless predicate reached from another rule remains a live tail query.
	if len(c.query.rules) == 0 && len(c.query.uses) == 0 {
		return c.finishEarlyUnreachable(fromLevel)
	}
	truth, err := c.manager.MkTrue()
	if err != nil {
		return lbool.Undef, err
	}
	root, err := c.query.NewProofObligation(nil, fromLevel, 0, truth, nil)
	if err != nil {
		return lbool.Undef, err
	}
	if err = c.queue.SetRoot(root); err != nil {
		return lbool.Undef, err
	}
	c.solved = true
	level := fromLevel
	for attempt := uint32(0); attempt < maxLevels; attempt++ {
		if err := spacerCheckpoint(c.manager); err != nil {
			return lbool.Undef, err
		}
		c.stats.maxQueryLevel = level
		c.stats.queryLevelSet = true
		reachable, err := c.queue.checkReachability(c.resetQueue, c.flexibleTrace, c.flexibleDepth, c.useRestarts, c.restartInitial, c.lemmaAdditions)
		if err != nil {
			return c.finishSearchFailure(err)
		}
		if reachable {
			c.lastResult = lbool.True
			if err = watch.Close(); err != nil {
				return c.lastResult, err
			}
			if err = c.validate(); err != nil {
				return c.lastResult, err
			}
			c.cexDepth, err = c.counterexampleDepth()
			if err == nil {
				c.cexDepthSet = true
			}
			return c.lastResult, err
		}
		if level > 0 && !c.skipPropagate {
			inductive, err := c.propagate(c.queue.ExpandedLevel(), level)
			if err != nil {
				return lbool.Undef, err
			}
			if inductive {
				if err = c.simplifyFormulas(); err != nil {
					return lbool.Undef, err
				}
				c.lastResult = lbool.False
				if err = watch.Close(); err != nil {
					return c.lastResult, err
				}
				return c.lastResult, c.validate()
			}
		}
		if err = c.queue.IncrementLevel(); err != nil {
			return lbool.Undef, err
		}
		level = c.queue.MaximumLevel()
		if level > c.stats.maxDepth {
			c.stats.maxDepth = level
		}
	}
	c.lastResult = lbool.Undef
	c.bounded = true
	return c.lastResult, nil
}

func (c *Context) LastResult() (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	return c.lastResult, nil
}

func (c *Context) inductiveProperty() (_ *InductiveProperty, resultErr error) {
	relations := make([]RelationInfo, 0, len(c.order))
	bodies := make([]ast.ASTRef, 0, len(c.order))
	defer func() {
		for index := len(bodies) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&bodies[index]))
		}
	}()
	for _, declaration := range c.order {
		if declaration == c.queryDecl {
			continue
		}
		transformer := c.transformers[declaration]
		bodyRef, err := transformer.Formulas(c.inductive, false)
		if err != nil {
			return nil, err
		}
		bodies = append(bodies, bodyRef)
		body, err := c.manager.RefGet(bodyRef)
		if err != nil {
			return nil, err
		}
		converted, err := c.owner.FormulaNewToOld(body, 0, false)
		if err != nil {
			return nil, err
		}
		bodies = append(bodies, converted)
		body, err = c.manager.RefGet(converted)
		if err != nil {
			return nil, err
		}
		signature, err := transformer.Signature()
		if err != nil {
			return nil, err
		}
		relations = append(relations, RelationInfo{Predicate: declaration, Variables: signature, Body: body})
	}
	return NewInductiveProperty(c.manager, relations)
}

// Model returns the inductive predicate interpretations for an unreachable
// query. The caller closes the returned model.
func (c *Context) Model(declaration ast.ASTHandle) (_ *model.Model, resultErr error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if !c.solved || c.lastResult != lbool.False || c.queryDecl != declaration {
		return nil, ast.ErrASTDomain
	}
	property, err := c.inductiveProperty()
	if err != nil {
		return nil, err
	}
	attempt := &contextExportAttempt{manager: c.manager, runtime: c.manager.NumeralService().RuntimeOwner(), property: property}
	attempt.model, err = property.Model()
	if err = attempt.finish(err); err != nil {
		return nil, err
	}
	return attempt.model, nil
}

// Answer returns the source ground reach-fact trace for a reachable query or
// the inductive predicate definitions for an unreachable query.
func (c *Context) Answer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if c.query == nil || c.queryDecl != declaration {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	if c.lastResult == lbool.True {
		return c.groundSatAnswer()
	}
	if c.lastResult == lbool.False {
		property, err := c.inductiveProperty()
		if err != nil {
			return ast.ASTRef{}, err
		}
		attempt := &contextExportAttempt{manager: c.manager, runtime: c.manager.NumeralService().RuntimeOwner(), property: property}
		attempt.answer, err = property.Expr()
		if err = attempt.finish(err); err != nil {
			return ast.ASTRef{}, err
		}
		return attempt.answer, nil
	}
	truth, err := c.manager.MkTrue()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.manager.NewRef(truth)
}

// GroundSatAnswer returns the concrete reach-fact explanation for a SAT
// query. The retained context returns an empty expression reference, rather
// than an operational error, when no satisfying answer is available.
func (c *Context) GroundSatAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if c.query == nil || c.queryDecl != declaration {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	if c.lastResult != lbool.True {
		return ast.ASTRef{}, writeSpacerVerbose(0, "Sat answer unavailable when result is false\n")
	}
	return c.groundSatAnswer()
}

func (c *Context) CollectStatistics(destination *statistics.Statistics) error {
	return c.CollectStatisticsWithSnapshot(destination, StatisticsSnapshot{})
}

// CollectStatisticsWithSnapshot keeps retired context/solver counters while
// collecting property and propagation gauges only from the active program.
func (c *Context) CollectStatisticsWithSnapshot(destination *statistics.Statistics, previous StatisticsSnapshot) error {
	if destination == nil {
		return ast.ErrASTHandle
	}
	current, err := c.SnapshotStatistics()
	if err != nil {
		return err
	}
	// Clock and diagnostic writes can fail in Go. Publish the complete source
	// contribution once, so callers can retry without duplicating a prefix.
	var pending statistics.Statistics
	previous.Merge(current)
	if err = previous.Collect(&pending); err != nil {
		return err
	}
	for _, declaration := range c.order {
		if err = c.transformers[declaration].CollectStatistics(&pending); err != nil {
			return err
		}
	}
	pending.UpdateUint("SPACER inductive level", c.inductive)
	if err := writeSpacerBrunchStatistics(previous, c.inductive); err != nil {
		return err
	}
	destination.Copy(pending)
	return nil
}

func (c *Context) ResetStatistics() error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.owner.ResetStatistics(); err != nil {
		return err
	}
	for _, declaration := range c.order {
		if err := c.transformers[declaration].ResetStatistics(); err != nil {
			return err
		}
	}
	c.queue.restarts = 0
	c.cexDepth = 0
	c.cexDepthSet = false
	c.stats = contextStatistics{}
	c.generalizerStats = boolGeneralizerStatistics{}
	c.timers.reset()
	return nil
}

// Reset translates context::reset without replacing its manager or selected
// query declaration. New rules can then be installed into the same context.
func (c *Context) Reset() (result error) {
	if err := c.open(); err != nil {
		return err
	}
	c.resetting = true
	c.queue = ProofObligationQueue{}
	c.query = nil
	c.lastResult = lbool.Undef
	c.inductive = 0
	c.solved = false
	c.bounded = false
	// Keep every old predicate's cover until all allocation-capable solver
	// retirements finish. update_rules can then expose or inherit those covers
	// again when a virtual solver's disabled-guard assertion refuses allocation.
	for index := len(c.order) - 1; index >= 0; index-- {
		if transformer := c.transformers[c.order[index]]; transformer != nil {
			if err := transformer.closeSolvers(); err != nil {
				return err
			}
		}
	}
	for index := len(c.order) - 1; index >= 0; index-- {
		declaration := c.order[index]
		if transformer := c.transformers[declaration]; transformer != nil {
			if err := transformer.Close(); err != nil {
				result = errors.Join(result, err)
			} else {
				c.transformers[declaration] = nil
			}
		}
	}
	if result != nil {
		return result
	}
	c.transformers = make(map[ast.ASTHandle]*PredicateTransformer)
	c.order = nil
	c.initialized = false
	c.resetting = false
	return nil
}

func (c *Context) Close() (result error) {
	if c == nil {
		return ErrContextOwner
	}
	if c.closed {
		return nil
	}
	if c.manager == nil {
		return ErrContextOwner
	}
	for index := len(c.order) - 1; index >= 0; index-- {
		declaration := c.order[index]
		if transformer := c.transformers[declaration]; transformer != nil {
			if err := transformer.Close(); err != nil {
				result = errors.Join(result, err)
			} else {
				c.transformers[declaration] = nil
			}
		}
	}
	if result == nil && c.queryRef != (ast.ASTRef{}) {
		result = c.manager.DelRef(&c.queryRef)
	}
	if result == nil {
		result = c.owner.Close()
	}
	if result == nil {
		c.manager = nil
		c.owner = nil
		c.transformers = nil
		c.order = nil
		c.query = nil
		c.closed = true
	}
	return result
}
