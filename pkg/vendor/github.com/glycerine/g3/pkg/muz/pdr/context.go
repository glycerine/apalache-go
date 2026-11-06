package pdr

// Source: retained context construction and init_rules orchestration in
// src/muz/pdr/pdr_context.h and pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrContextOwner = errors.New("muz/pdr: invalid or closed context")

// Context owns the PDR manager and the predicate transformers for one Horn
// rule set. The conflict/search loop is layered on this initialized program.
type Context struct {
	manager                  *ast.ASTManager
	owner                    *Manager
	cacheMode                ReachableCacheMode
	transformers             map[ast.ASTHandle]*PredicateTransformer
	auxiliary                map[ast.ASTHandle]bool
	order                    []ast.ASTHandle
	search                   *ModelSearch
	implicant                *model.ModelImplicant
	lite                     *qe.Lite
	projectionTheory         *rewriter.TheoryRewriter
	projectionTheoryReady    bool
	projectionTheoryBorrowed bool
	cooperation              *cooperate.Cooperation
	ownedCoop                bool
	lastResult               lbool.LBool
	unknownResult            bool // a successful unknown solve, not an operational failure
	query                    *PredicateTransformer
	queryDecl                ast.ASTHandle
	expanded                 uint32
	inductive                uint32
	nodes                    uint32
	maxDepth                 uint32
	useFarkas                bool
	farkasActive             bool
	proofActive              bool
	useModel                 bool
	useInductive             bool
	useMultiCore             bool
	useArithInductive        bool
	useInduction             bool
	useConvexClosure         bool
	useConvexInterior        bool
	useUTVPI                 bool
	flexibleTrace            bool
	fullTraceAnswer          bool
	generateProofTrace       bool
	printBoogieCert          bool
	certificateModel         CertificateModelConverter
	validateResult           bool
	simplifyPre              bool
	simplifyPost             bool
	initialized              bool
	closed                   bool
}

// CertificateModelConverter consumes a normalized finite-frame model and
// returns the source-signature model used by context::display_certificate.
// On failure it owns cleanup of the transferred input model.
type CertificateModelConverter interface {
	ConvertCertificateModel(source *model.Model) (*model.Model, error)
}

// SetCertificateModelConverter installs the live fixedpoint preprocessing
// converter before rule installation. Context borrows the converter owner.
func (c *Context) SetCertificateModelConverter(converter CertificateModelConverter) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.certificateModel = converter
	return nil
}

// SetPrintBoogieCertificate selects the source reachable-result verbose
// certificate operand. It must be fixed before rule installation.
func (c *Context) SetPrintBoogieCertificate(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.printBoogieCert = enabled
	return nil
}

// SetResultValidation selects source post-solve trace, proof, and inductive-
// model validation. It must be fixed before rule installation.
func (c *Context) SetResultValidation(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.validateResult = enabled
	return nil
}

// SetFrameSimplification selects the source unit-subsumption hooks around
// each inductive propagation pass. It must be fixed before initialization.
func (c *Context) SetFrameSimplification(before, after bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.simplifyPre, c.simplifyPost = before, after
	return nil
}

func NewContext(manager *ast.ASTManager, params *uparams.ParamsRef, maximumContexts uint32, cacheMode ReachableCacheMode, config smtcore.SMTSolverConfig) (*Context, error) {
	return newContext(manager, params, maximumContexts, cacheMode, config, nil)
}

// NewContextWithTheory borrows the fixedpoint context's existing source
// rewriter. The caller must keep it open until the PDR context is closed.
func NewContextWithTheory(manager *ast.ASTManager, params *uparams.ParamsRef, maximumContexts uint32, cacheMode ReachableCacheMode, config smtcore.SMTSolverConfig, theory *rewriter.TheoryRewriter) (*Context, error) {
	if theory == nil || theory.Manager() != manager {
		return nil, ErrContextOwner
	}
	if _, err := theory.NumSteps(); err != nil {
		return nil, err
	}
	return newContext(manager, params, maximumContexts, cacheMode, config, theory)
}

func newContext(manager *ast.ASTManager, params *uparams.ParamsRef, maximumContexts uint32, cacheMode ReachableCacheMode, config smtcore.SMTSolverConfig, theory *rewriter.TheoryRewriter) (*Context, error) {
	if manager == nil || cacheMode > ReachableConstraintCache {
		return nil, ErrContextOwner
	}
	owner, err := NewManager(manager, params, maximumContexts, config)
	if err != nil {
		return nil, err
	}
	result := &Context{manager: manager, owner: owner, cacheMode: cacheMode, search: NewModelSearch(true), lastResult: lbool.Undef, useInductive: true, useUTVPI: true, auxiliary: make(map[ast.ASTHandle]bool)}
	if theory != nil {
		result.projectionTheory = theory
		result.projectionTheoryReady = true
		result.projectionTheoryBorrowed = true
	}
	fail := func(cause error) (*Context, error) {
		cleanup := result.Close()
		if cleanup != nil {
			return result, errors.Join(cause, cleanup)
		}
		return nil, cause
	}
	if config.Globals == nil {
		return result, nil
	}
	// Source dl_context constructs its default theory rewriter eagerly, so
	// capture global options at context construction rather than first use.
	if err = result.ensureProjectionTheory(); err != nil {
		return fail(err)
	}
	result.cooperation = config.ModelEvaluator.Cooperation
	if result.cooperation == nil {
		result.cooperation, err = cooperate.NewCooperation(manager.Budget())
		if err != nil {
			return fail(err)
		}
		result.ownedCoop = true
	}
	services := config.ModelEvaluator
	services.Context = config.Params
	services.Globals = config.Globals
	services.Memory = manager.NumeralService().RuntimeOwner()
	services.Cooperation = result.cooperation
	result.owner.contexts.config.ModelEvaluator = services
	result.implicant, err = model.NewModelImplicant(manager, services)
	if err != nil {
		return fail(err)
	}
	result.lite, err = qe.NewLite(manager, params, true, qe.Config{Context: config.Params, Globals: config.Globals, Cooperation: result.cooperation, Trace: config.ModelEvaluator.Trace})
	if err != nil {
		return fail(err)
	}
	return result, nil
}

func (c *Context) open() error {
	if c == nil || c.closed || c.manager == nil || c.owner == nil {
		return ErrContextOwner
	}
	return c.owner.open()
}

// Initialized reports whether an executable rule set has been installed.
func (c *Context) Initialized() (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	return c.initialized, nil
}

func (c *Context) SetBackground(background ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	return c.owner.SetBackground(background)
}

// SetUseFarkas selects source proof-derived arithmetic cores. It must be set
// before Initialize, because proof mode is fixed before child SMT solvers are
// constructed.
func (c *Context) SetUseFarkas(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.useFarkas = enabled
	return nil
}

// SetUseInductiveGeneralizer selects the source Boolean inductive core
// minimizer. It must be set before Initialize.
func (c *Context) SetUseInductiveGeneralizer(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.useInductive = enabled
	return nil
}

// SetUseMultiCoreGeneralizer selects the source multi-core wrapper. The
// retained source implementation minimizes and emits its first core; its
// additional-core loop observes an empty old-core vector.
func (c *Context) SetUseMultiCoreGeneralizer(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.useMultiCore = enabled
	return nil
}

// SetUseArithmeticInductiveGeneralizer selects the source experimental
// arithmetic core extrapolator. It must be set before Initialize.
func (c *Context) SetUseArithmeticInductiveGeneralizer(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.useArithInductive = enabled
	return nil
}

// SetUseInductiveReachabilityGeneralizer selects the source bounded induction
// check over parent obligations. It must be set before Initialize.
func (c *Context) SetUseInductiveReachabilityGeneralizer(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.useInduction = enabled
	return nil
}

// SetUseConvexClosureGeneralizer selects the source closed convex-hull core
// generalizer. It must be set before Initialize.
func (c *Context) SetUseConvexClosureGeneralizer(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.useConvexClosure = enabled
	return nil
}

// SetUseConvexInteriorGeneralizer selects the source open convex-hull core
// generalizer. It must be set before Initialize.
func (c *Context) SetUseConvexInteriorGeneralizer(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.useConvexInterior = enabled
	return nil
}

// SetUseUTVPI selects source PDR arithmetic-fragment specialization. It must
// be set before Initialize because child SMT backends are selected there.
func (c *Context) SetUseUTVPI(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.useUTVPI = enabled
	return nil
}

// SetUseModelGeneralizer selects model-cone backward propagation instead of
// literal simulation. It must be set before Initialize.
func (c *Context) SetUseModelGeneralizer(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.useModel = enabled
	return nil
}

// SetBreadthFirstModelSearch selects the source PDR obligation-frontier
// strategy. It must be set before Initialize.
func (c *Context) SetBreadthFirstModelSearch(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized || c.search == nil {
		return ast.ErrASTDomain
	}
	c.search.breadthFirst = enabled
	return nil
}

func (c *Context) BreadthFirstModelSearch() (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if c.search == nil {
		return false, ast.ErrASTDomain
	}
	return c.search.breadthFirst, nil
}

// SetFlexibleTrace selects source level-raising backtracking for blocking
// cores that still depend on the current frame. It must be set before
// Initialize.
func (c *Context) SetFlexibleTrace(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.flexibleTrace = enabled
	return nil
}

// SetFullTraceAnswer retains the query predicate and every reached predicate
// when no fixedpoint rule transformation will supply an answer converter.
func (c *Context) SetFullTraceAnswer(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.fullTraceAnswer = enabled
	return nil
}

// SetGenerateProofTrace selects the source reachable-answer proof object.
// It must be set before Initialize so answer selection is fixed for the query
// context. Proof mode itself is scoped around reconstruction after solving.
func (c *Context) SetGenerateProofTrace(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	c.generateProofTrace = enabled
	return nil
}

// SetAuxiliaryPredicates records declarations introduced by retained rule
// transformations. Their transformer state participates in solving but is
// omitted from externally visible inductive certificates, as in the source
// model-converter path. It must be called before Initialize.
func (c *Context) SetAuxiliaryPredicates(declarations []ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized {
		return ast.ErrASTDomain
	}
	for _, declaration := range declarations {
		kind, err := c.manager.GetKind(declaration)
		if err != nil {
			return err
		}
		if kind != ast.ASTFuncDecl {
			return ast.ErrASTDomain
		}
		c.auxiliary[declaration] = true
	}
	return nil
}

func (c *Context) FarkasActive() (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	return c.farkasActive, nil
}

// IsDifferenceLogic reports whether Initialize selected the source PDR
// difference-logic arithmetic backend.
func (c *Context) IsDifferenceLogic() (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if !c.initialized {
		return false, ast.ErrASTDomain
	}
	return c.owner.contexts.arithmeticMode() == smtcore.SMTArithmeticDifferenceLogic, nil
}

// IsUTVPI reports whether Initialize selected the source PDR UTVPI backend.
func (c *Context) IsUTVPI() (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if !c.initialized {
		return false, ast.ErrASTDomain
	}
	return c.owner.contexts.arithmeticMode() == smtcore.SMTArithmeticUTVPI, nil
}

func (c *Context) enterPDRProofMode() (func() error, error) {
	previous, err := c.manager.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	desired := ast.ASTProofDisabled
	if c.proofActive {
		desired = ast.ASTProofEnabled
	}
	if previous != c.proofActive {
		if err = c.manager.ToggleProofMode(desired); err != nil {
			return nil, err
		}
	}
	return func() error {
		if previous == c.proofActive {
			return nil
		}
		if previous {
			return c.manager.ToggleProofMode(ast.ASTProofEnabled)
		}
		return c.manager.ToggleProofMode(ast.ASTProofDisabled)
	}, nil
}

func appendDeclaration(order *[]ast.ASTHandle, seen map[ast.ASTHandle]bool, declaration ast.ASTHandle) {
	if !seen[declaration] {
		seen[declaration] = true
		*order = append(*order, declaration)
	}
}

func closeRuleBuckets(buckets map[ast.ASTHandle][]*base.Rule) (result error) {
	for _, rules := range buckets {
		for _, rule := range rules {
			result = errors.Join(result, rule.Close())
		}
	}
	return result
}

// Initialize builds a transformer for every head or predicate-tail
// declaration, then grounds each head's rules into its transition system.
func (c *Context) Initialize(set *base.RuleSet) (resultErr error) {
	if err := c.open(); err != nil {
		return err
	}
	if c.initialized || set == nil {
		return ast.ErrASTDomain
	}
	plan, err := c.planRules(set)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, plan.close()) }()
	order, buckets := plan.order, plan.buckets
	transformers := make(map[ast.ASTHandle]*PredicateTransformer, len(order))
	// A pre-query cover may have allocated solvers in the old proof mode.
	// Build rule transformers from a fresh pool; the manager continues to own
	// the old solvers until Close, while their frame properties are inherited.
	c.owner.contexts.solverStart = len(c.owner.contexts.solvers)
	c.owner.contexts.arithmetic.enabled = false
	c.owner.contexts.arithmetic.effective = smtcore.SMTArithmeticNoSolver
	nonBoolean := plan.nonBoolean
	c.farkasActive = c.useFarkas && nonBoolean
	c.proofActive = nonBoolean || c.useConvexClosure || c.useConvexInterior
	arithmetic := plan.arithmetic
	if arithmetic.enabled {
		if err = c.owner.contexts.configureArithmetic(arithmetic.selectMode, arithmetic.mode, arithmetic.eq2ineq); err != nil {
			return err
		}
	}
	restoreProofMode, err := c.enterPDRProofMode()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, restoreProofMode()) }()
	committed := false
	defer func() {
		if committed {
			return
		}
		for index := len(order) - 1; index >= 0; index-- {
			if transformer := transformers[order[index]]; transformer != nil {
				resultErr = errors.Join(resultErr, transformer.Close())
			}
		}
	}()
	for _, declaration := range order {
		transformer, x := NewPredicateTransformer(c.owner, declaration, c.cacheMode)
		if x != nil {
			return x
		}
		transformers[declaration] = transformer
		if err = transformer.solver.SetUseFarkas(c.farkasActive); err != nil {
			return err
		}
	}
	for _, declaration := range order {
		if err = transformers[declaration].InitializeRules(transformers, buckets[declaration]); err != nil {
			return fmt.Errorf("muz/pdr: initialize predicate rules: %w", err)
		}
		if prior := c.transformers[declaration]; prior != nil {
			if err = transformers[declaration].InheritProperties(prior); err != nil {
				return fmt.Errorf("muz/pdr: inherit pre-query properties: %w", err)
			}
		}
	}
	if err = closeTransformers(c.order, c.transformers); err != nil {
		return err
	}
	c.transformers = transformers
	c.order = order
	c.initialized = true
	committed = true
	return nil
}

func (c *Context) Transformer(declaration ast.ASTHandle) (*PredicateTransformer, bool, error) {
	if err := c.open(); err != nil {
		return nil, false, err
	}
	if !c.initialized {
		return nil, false, ast.ErrASTDomain
	}
	transformer, found := c.transformers[declaration]
	return transformer, found, nil
}

// Property operations are valid before initial rule installation in the source.
func (c *Context) propertyTransformer(declaration ast.ASTHandle) (*PredicateTransformer, bool, error) {
	if err := c.open(); err != nil {
		return nil, false, err
	}
	transformer, found := c.transformers[declaration]
	return transformer, found, nil
}

// NumLevels reports the number of finite frames retained for a predicate.
func (c *Context) NumLevels(declaration ast.ASTHandle) (uint32, error) {
	transformer, found, err := c.propertyTransformer(declaration)
	if err != nil || !found {
		return 0, err
	}
	return transformer.NumLevels()
}

// CoverDelta returns one predicate cover frame as an owning AST reference.
// Level -1 selects the invariant frame.
func (c *Context) CoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	transformer, found, err := c.propertyTransformer(declaration)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !found {
		truth, x := c.manager.MkTrue()
		if x != nil {
			return ast.ASTRef{}, x
		}
		return c.manager.NewRef(truth)
	}
	return transformer.CoverDelta(level)
}

// AddCover adds a bound-variable property to one predicate frame.
func (c *Context) AddCover(level int, declaration, property ast.ASTHandle) error {
	transformer, found, err := c.propertyTransformer(declaration)
	if err != nil {
		return err
	}
	if !found {
		transformer, err = NewPredicateTransformer(c.owner, declaration, c.cacheMode)
		if err != nil {
			return err
		}
		if c.transformers == nil {
			c.transformers = make(map[ast.ASTHandle]*PredicateTransformer)
		}
		c.transformers[declaration] = transformer
		c.order = append(c.order, declaration)
	}
	return transformer.AddCover(level, property)
}

func (c *Context) queryState(query ast.ASTHandle) (*PredicateTransformer, ast.ASTHandle, []ast.ASTRef, error) {
	references := make([]ast.ASTRef, 0)
	fail := func(err error) (*PredicateTransformer, ast.ASTHandle, []ast.ASTRef, error) {
		for index := len(references) - 1; index >= 0; index-- {
			err = errors.Join(err, c.manager.DelRef(&references[index]))
		}
		return nil, ast.ASTHandle{}, nil, err
	}
	kind, err := c.manager.GetKind(query)
	if err != nil || kind != ast.ASTApp {
		if err != nil {
			return fail(err)
		}
		return fail(ast.ErrASTDomain)
	}
	declaration, err := c.manager.GetDecl(query)
	if err != nil {
		return fail(err)
	}
	transformer := c.transformers[declaration]
	if transformer == nil {
		return fail(ast.ErrASTDomain)
	}
	count, err := c.manager.GetNumArgs(query)
	if err != nil || count != uint32(len(transformer.signature)) {
		if err != nil {
			return fail(err)
		}
		return fail(ast.ErrASTDomain)
	}
	equalities := make([]ast.ASTHandle, count)
	for index := uint32(0); index < count; index++ {
		stateDeclaration, x := c.owner.OldToNew(transformer.signature[index], 0)
		if x != nil {
			return fail(x)
		}
		state, x := c.manager.MkApp(stateDeclaration, nil)
		if x != nil {
			return fail(x)
		}
		argument, x := c.manager.GetArg(query, index)
		if x != nil {
			return fail(x)
		}
		equalities[index], x = c.manager.MkEq(state, argument)
		if x != nil {
			return fail(x)
		}
		reference, x := c.manager.NewRef(equalities[index])
		if x != nil {
			return fail(x)
		}
		references = append(references, reference)
	}
	state, err := c.manager.MkAndArray(equalities)
	if err != nil {
		return fail(err)
	}
	reference, err := c.manager.NewRef(state)
	if err != nil {
		return fail(err)
	}
	references = append(references, reference)
	return transformer, state, references, nil
}

// CheckInitial tests a concrete predicate application against level zero. It
// is the narrow executable query path used before the full model-search loop.
func (c *Context) CheckInitial(query ast.ASTHandle, wantModel bool) (PropCheckResult, error) {
	if err := c.open(); err != nil {
		return PropCheckResult{}, err
	}
	if !c.initialized {
		return PropCheckResult{}, ast.ErrASTDomain
	}
	transformer, state, references, err := c.queryState(query)
	if err != nil {
		return PropCheckResult{}, err
	}
	result, checkErr := transformer.CheckReachable(0, state, false, wantModel)
	for index := len(references) - 1; index >= 0; index-- {
		checkErr = errors.Join(checkErr, c.manager.DelRef(&references[index]))
	}
	return result, checkErr
}

func (c *Context) InitialStatus(query ast.ASTHandle) (lbool.LBool, error) {
	result, err := c.CheckInitial(query, false)
	return result.Status, err
}

func (c *Context) CollectStatistics(destination *statistics.Statistics) error {
	return c.CollectStatisticsWithSnapshot(destination, StatisticsSnapshot{})
}

// CollectStatisticsWithSnapshot combines prior query counters with this live
// program, preserving maximum depth and current transformer gauges.
func (c *Context) CollectStatisticsWithSnapshot(destination *statistics.Statistics, previous StatisticsSnapshot) error {
	if err := c.open(); err != nil {
		return err
	}
	if destination == nil {
		return ErrContextOwner
	}
	// A backend can refuse collection while completing a failed scope pop.
	// Keep the earlier transformer and archived contributions local so a
	// retry cannot publish their prefix more than once.
	var pending statistics.Statistics
	pending.Copy(previous.Solvers)
	for _, declaration := range c.order {
		if err := c.transformers[declaration].CollectStatistics(&pending); err != nil {
			return err
		}
	}
	if err := c.owner.CollectStatistics(&pending); err != nil {
		return err
	}
	pending.UpdateUint("PDR num unfoldings", c.nodes+previous.Unfoldings)
	depth := c.maxDepth
	if previous.MaxDepth > depth {
		depth = previous.MaxDepth
	}
	pending.UpdateUint("PDR max depth", depth)
	pending.UpdateUint("PDR inductive level", c.inductive)
	destination.Copy(pending)
	return nil
}

func (c *Context) ResetStatistics() error {
	if err := c.open(); err != nil {
		return err
	}
	for _, declaration := range c.order {
		if err := c.transformers[declaration].ResetStatistics(); err != nil {
			return err
		}
	}
	if err := c.owner.ResetStatistics(); err != nil {
		return err
	}
	c.nodes, c.maxDepth = 0, 0
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
	if c.search != nil {
		if err := c.search.Close(); err != nil {
			return err
		}
		c.search = nil
	}
	if c.implicant != nil {
		if err := c.implicant.Close(); err != nil {
			return err
		}
		c.implicant = nil
	}
	if c.lite != nil {
		if err := c.lite.Close(); err != nil {
			return err
		}
		c.lite = nil
	}
	if c.projectionTheory != nil {
		if !c.projectionTheoryBorrowed {
			if err := c.projectionTheory.Close(); err != nil {
				return err
			}
		}
		c.projectionTheory = nil
		c.projectionTheoryReady = false
		c.projectionTheoryBorrowed = false
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
		result = errors.Join(result, c.owner.Close())
	}
	if result == nil && c.ownedCoop && c.cooperation != nil {
		result = c.cooperation.Close()
		if result == nil {
			c.cooperation = nil
			c.ownedCoop = false
		}
	}
	if result == nil {
		c.manager = nil
		c.owner = nil
		c.transformers = nil
		c.auxiliary = nil
		c.order = nil
		c.search = nil
		c.implicant = nil
		c.lite = nil
		c.cooperation = nil
		c.query = nil
		c.certificateModel = nil
		c.closed = true
	}
	return result
}
