package pdr

// Source: retained context construction and init_rules orchestration in
// src/muz/pdr/pdr_context.h and pdr_context.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
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
	manager           *ast.ASTManager
	owner             *Manager
	cacheMode         ReachableCacheMode
	transformers      map[ast.ASTHandle]*PredicateTransformer
	auxiliary         map[ast.ASTHandle]bool
	order             []ast.ASTHandle
	search            *ModelSearch
	implicant         *model.ModelImplicant
	lite              *qe.Lite
	cooperation       *cooperate.Cooperation
	ownedCoop         bool
	lastResult        lbool.LBool
	query             *PredicateTransformer
	queryDecl         ast.ASTHandle
	expanded          uint32
	inductive         uint32
	nodes             uint32
	maxDepth          uint32
	useFarkas         bool
	farkasActive      bool
	proofActive       bool
	useModel          bool
	useInductive      bool
	useConvexClosure  bool
	useConvexInterior bool
	flexibleTrace     bool
	fullTraceAnswer   bool
	initialized       bool
	closed            bool
}

func NewContext(manager *ast.ASTManager, params *uparams.ParamsRef, maximumContexts uint32, cacheMode ReachableCacheMode, config smtcore.SMTSolverConfig) (*Context, error) {
	if manager == nil || cacheMode > ReachableConstraintCache {
		return nil, ErrContextOwner
	}
	owner, err := NewManager(manager, params, maximumContexts, config)
	if err != nil {
		return nil, err
	}
	result := &Context{manager: manager, owner: owner, cacheMode: cacheMode, search: NewModelSearch(true), lastResult: lbool.Undef, useInductive: true, auxiliary: make(map[ast.ASTHandle]bool)}
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

func (c *Context) enterPDRProofMode() (func() error, error) {
	if !c.proofActive {
		return func() error { return nil }, nil
	}
	previous, err := c.manager.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	if !previous {
		if err = c.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
			return nil, err
		}
	}
	return func() error {
		if previous {
			return nil
		}
		return c.manager.ToggleProofMode(ast.ASTProofDisabled)
	}, nil
}

func (c *Context) declarationsUseNonBooleanSort(order []ast.ASTHandle) (bool, error) {
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	for _, declaration := range order {
		arity, err := c.manager.GetArity(declaration)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < arity; i++ {
			sort, err := c.manager.GetDomain(declaration, i)
			if err != nil {
				return false, err
			}
			if sort != boolean {
				return true, nil
			}
		}
	}
	return false, nil
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
	count, err := set.Len()
	if err != nil {
		return err
	}
	order := make([]ast.ASTHandle, 0)
	seen := make(map[ast.ASTHandle]bool)
	buckets := make(map[ast.ASTHandle][]*base.Rule)
	defer func() { resultErr = errors.Join(resultErr, closeRuleBuckets(buckets)) }()
	for index := 0; index < count; index++ {
		rule, x := set.Rule(index)
		if x != nil {
			return x
		}
		head, x := rule.HeadDeclaration()
		if x != nil {
			return errors.Join(x, rule.Close())
		}
		appendDeclaration(&order, seen, head)
		buckets[head] = append(buckets[head], rule)
		uninterpreted, x := rule.UninterpretedTailSize()
		if x != nil {
			return x
		}
		for tail := 0; tail < uninterpreted; tail++ {
			declaration, y := rule.TailDeclaration(tail)
			if y != nil {
				return y
			}
			appendDeclaration(&order, seen, declaration)
		}
	}
	transformers := make(map[ast.ASTHandle]*PredicateTransformer, len(order))
	nonBoolean, err := c.declarationsUseNonBooleanSort(order)
	if err != nil {
		return err
	}
	c.farkasActive = c.useFarkas && nonBoolean
	c.proofActive = c.farkasActive || c.useConvexClosure || c.useConvexInterior
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
			return err
		}
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

// NumLevels reports the number of finite frames retained for a predicate.
func (c *Context) NumLevels(declaration ast.ASTHandle) (uint32, error) {
	transformer, found, err := c.Transformer(declaration)
	if err != nil || !found {
		return 0, err
	}
	return transformer.NumLevels()
}

// CoverDelta returns one predicate cover frame as an owning AST reference.
// Level -1 selects the invariant frame.
func (c *Context) CoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	transformer, found, err := c.Transformer(declaration)
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
	transformer, found, err := c.Transformer(declaration)
	if err != nil {
		return err
	}
	if !found {
		transformer, err = NewPredicateTransformer(c.owner, declaration, c.cacheMode)
		if err != nil {
			return err
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
	if err := c.open(); err != nil {
		return err
	}
	if destination == nil {
		return ErrContextOwner
	}
	for _, declaration := range c.order {
		if err := c.transformers[declaration].CollectStatistics(destination); err != nil {
			return err
		}
	}
	if err := c.owner.CollectStatistics(destination); err != nil {
		return err
	}
	destination.UpdateUint("PDR num unfoldings", c.nodes)
	destination.UpdateUint("PDR max depth", c.maxDepth)
	destination.UpdateUint("PDR inductive level", c.inductive)
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
	c.nodes, c.maxDepth, c.inductive = 0, 0, 0
	return nil
}

func (c *Context) Close() (result error) {
	if c == nil || c.manager == nil {
		return ErrContextOwner
	}
	if c.closed {
		return nil
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
		c.closed = true
	}
	return result
}
