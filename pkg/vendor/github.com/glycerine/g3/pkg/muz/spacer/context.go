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
)

var ErrContextOwner = errors.New("muz/spacer: invalid or closed context")

type Context struct {
	manager          *ast.ASTManager
	owner            *Manager
	config           PropSolverConfig
	transformers     map[ast.ASTHandle]*PredicateTransformer
	order            []ast.ASTHandle
	query            *PredicateTransformer
	queryDecl        ast.ASTHandle
	queue            ProofObligationQueue
	lastResult       lbool.LBool
	inductive        uint32
	skipPropagate    bool
	resetQueue       bool
	flexibleTrace    bool
	flexibleDepth    uint32
	reusePOBs        bool
	simplifyPOBs     bool
	blastTermITE     bool
	initReachFacts   bool
	reachAsInitial   bool
	useDerivations   bool
	reverseChildren  bool
	eliminateAux     bool
	reachDNF         bool
	useRestarts      bool
	restartInitial   uint32
	instantiate      bool
	quantifiedLemmas bool
	groundCTI        bool
	weakAbstraction  bool
	validateResult   bool
	simplifyPre      bool
	simplifyPost     bool
	initialized      bool
	solved           bool
	closed           bool
}

func NewContext(manager *ast.ASTManager, params *uparams.ParamsRef, maximumContexts uint32, config smtcore.SMTSolverConfig) (*Context, error) {
	if manager == nil {
		return nil, ErrContextOwner
	}
	owner, err := NewManager(manager, params, maximumContexts, config)
	if err != nil {
		return nil, err
	}
	propConfig, err := spacerPropSolverConfig(config)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	fixedpoint, err := config.Globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	skipName := "spacer.skip_propagate"
	skipPropagate, err := config.Params.GetBoolName(&fixedpoint, &skipName, false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	resetName := "spacer.reset_obligation_queue"
	resetQueue, err := config.Params.GetBoolName(&fixedpoint, &resetName, true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	flexibleName := "pdr.flexible_trace"
	flexibleTrace, err := config.Params.GetBoolName(&fixedpoint, &flexibleName, false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	depthName := "pdr.flexible_trace_depth"
	flexibleDepth, err := config.Params.GetUIntName(&fixedpoint, &depthName, InfinityLevel)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	getBool := func(name string, fallback bool) (bool, error) {
		return config.Params.GetBoolName(&fixedpoint, &name, fallback)
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
	orderName := "spacer.order_children"
	orderChildren, err := config.Params.GetUIntName(&fixedpoint, &orderName, 0)
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
	restartName := "spacer.restart_initial_threshold"
	restartInitial, err := config.Params.GetUIntName(&fixedpoint, &restartName, 10)
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
		simplifyPre:     simplifyPre,
		simplifyPost:    simplifyPost,
	}, nil
}

func spacerPropSolverConfig(config smtcore.SMTSolverConfig) (PropSolverConfig, error) {
	if config.Params == nil || config.Globals == nil || config.Globals.Context != config.Params {
		return PropSolverConfig{}, ErrContextOwner
	}
	fixedpoint, err := config.Globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return PropSolverConfig{}, err
	}
	get := func(name string, fallback bool) (bool, error) {
		return config.Params.GetBoolName(&fixedpoint, &name, fallback)
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
	if c.initialized {
		return ast.ErrASTDomain
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

func (c *Context) Initialize(set *base.RuleSet) (resultErr error) {
	return c.InitializePredicates(set, nil)
}

// InitializePredicates constructs transformers for every registered live
// predicate, including output predicates without defining rules.
func (c *Context) InitializePredicates(set *base.RuleSet, declarations []ast.ASTHandle) (resultErr error) {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized || set == nil {
		return ast.ErrASTDomain
	}
	count, err := set.Len()
	if err != nil {
		return err
	}
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
		return 0, ast.ErrASTDomain
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
	transformer := c.transformers[declaration]
	if transformer == nil {
		return ast.ErrASTDomain
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

func (c *Context) propagate(minimum, maximum uint32) (bool, error) {
	if minimum == InfinityLevel {
		return false, nil
	}
	if c.simplifyPre {
		if err := c.simplifyFormulas(); err != nil {
			return false, err
		}
	}
	for level := minimum; ; level++ {
		all := true
		for _, declaration := range c.order {
			propagated, err := c.transformers[declaration].PropagateToNextLevel(level)
			if err != nil {
				return false, err
			}
			all = all && propagated
		}
		if all {
			for _, declaration := range c.order {
				if err := c.transformers[declaration].PropagateToInfinity(level); err != nil {
					return false, err
				}
			}
			return level <= maximum, nil
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
	var result uint64
	for _, declaration := range c.order {
		count, err := c.transformers[declaration].frames.LemmaAdditions()
		if err != nil {
			return 0, err
		}
		result += count
	}
	return result, nil
}

// Solve runs bounded Spacer search starting at fromLevel. maxLevels is the
// number of search levels to attempt before returning undef.
func (c *Context) Solve(fromLevel, maxLevels uint32) (_ lbool.LBool, resultErr error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	if !c.initialized || c.query == nil || c.solved {
		return lbool.Undef, ast.ErrASTDomain
	}
	queryRules, err := c.query.Rules()
	if err != nil {
		return lbool.Undef, err
	}
	for _, rule := range queryRules {
		resultErr = errors.Join(resultErr, rule.Close())
	}
	if resultErr != nil {
		return lbool.Undef, resultErr
	}
	// context::solve_core returns false before constructing a root obligation
	// when preprocessing removed every definition of the query predicate.
	if len(queryRules) == 0 {
		c.solved = true
		c.inductive = fromLevel
		c.lastResult = lbool.False
		return c.lastResult, nil
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
		reachable, err := c.queue.checkReachability(c.resetQueue, c.flexibleTrace, c.flexibleDepth, c.useRestarts, c.restartInitial, c.lemmaAdditions)
		if err != nil {
			return lbool.Undef, err
		}
		if reachable {
			c.lastResult = lbool.True
			return c.lastResult, c.validate()
		}
		if level > 0 && !c.skipPropagate {
			inductive, err := c.propagate(c.queue.ExpandedLevel(), level)
			if err != nil {
				return lbool.Undef, err
			}
			if inductive {
				c.inductive = level
				if err = c.simplifyFormulas(); err != nil {
					return lbool.Undef, err
				}
				c.lastResult = lbool.False
				return c.lastResult, c.validate()
			}
		}
		if err = c.queue.IncrementLevel(); err != nil {
			return lbool.Undef, err
		}
		level = c.queue.MaximumLevel()
	}
	c.lastResult = lbool.Undef
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
		body, err := transformer.Formulas(c.inductive, false)
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
	defer func() { resultErr = errors.Join(resultErr, property.Close()) }()
	return property.Model()
}

// Answer returns the source ground reach-fact trace for a reachable query or
// the inductive predicate definitions for an unreachable query.
func (c *Context) Answer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if !c.solved || c.queryDecl != declaration || c.lastResult == lbool.Undef {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	if c.lastResult == lbool.True {
		return c.groundSatAnswer()
	}
	property, err := c.inductiveProperty()
	if err != nil {
		return ast.ASTRef{}, err
	}
	result, answerErr := property.Expr()
	return result, errors.Join(answerErr, property.Close())
}

func (c *Context) CollectStatistics(destination *statistics.Statistics) error {
	if err := c.open(); err != nil {
		return err
	}
	if destination == nil {
		return ast.ErrASTHandle
	}
	if err := c.owner.CollectStatistics(destination); err != nil {
		return err
	}
	for _, declaration := range c.order {
		if err := c.transformers[declaration].CollectStatistics(destination); err != nil {
			return err
		}
	}
	destination.UpdateUint64("SPACER restarts", c.queue.RestartCount())
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
	return nil
}

func (c *Context) Close() (result error) {
	if c == nil || c.manager == nil {
		return ErrContextOwner
	}
	if c.closed {
		return nil
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
