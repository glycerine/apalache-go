package api

// Source: src/api/api_interp.cpp, retained context, marker, and compute APIs.

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	_ "github.com/glycerine/g3/pkg/engine" // generated parameter collector owners
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

var (
	ErrInterpolationSAT     = errors.New("interpolation: formula is satisfiable")
	ErrInterpolationUnknown = errors.New("interpolation: result unknown")
)

// InterpolationContext owns a private proof-enabled AST context and its
// services. It is single-goroutine, like the native solver. Construct terms
// using Manager and Terms, and pin intermediate handles with ASTManager.NewRef
// when they must survive other operations. Markers and computed interpolants
// are retained by this context until Close, matching goivy's context lifetime.
// Copies share the same private state; closing any copy closes that state.
type InterpolationContext struct {
	*interpolationState
}

type interpolationState struct {
	budget      *arena.Budget
	symbols     *symbol.Manager
	numerals    *numeral.RationalService
	parameters  *params.ParamsContext
	catalog     *params.GlobalParamsCatalog
	globals     *params.GlobalParamsService
	cooperation *cooperate.Cooperation
	manager     *ast.ASTManager
	terms       *interp.Manager
	profiler    *interp.Profiler
	factory     solver.Factory
	evaluator   model.ModelEvaluatorConfig
	retained    []ast.ASTRef
	closed      bool
}

// InterpolationResult is the complete result of an interpolation solve.
// Interpolants are borrowed from the context until Close. Model is one owning
// reference when Status is satisfiable; the caller must call DecRef before
// closing the context.
type InterpolationResult struct {
	Status       lbool.LBool
	Interpolants []ast.ASTHandle
	Model        *model.ModelCore
}

// NewInterpolationContext is the native equivalent of
// Z3_mk_interpolation_context: proof and model production are enabled from
// construction. No state or symbol cache is shared with other contexts.
func NewInterpolationContext() (result *InterpolationContext, resultErr error) {
	c := &InterpolationContext{interpolationState: &interpolationState{budget: arena.NewBudget(1 << 40)}}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, c.Close())
		}
	}()
	var err error
	c.symbols, err = symbol.NewManager(symbol.Config{Budget: c.budget})
	if err != nil {
		return nil, err
	}
	c.numerals, err = numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: c.budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: c.budget}}}})
	if err != nil {
		return nil, err
	}
	c.parameters, err = params.NewParamsContext(c.symbols, c.numerals, params.ParamsConfig{Budget: c.budget})
	if err != nil {
		return nil, err
	}
	c.catalog, err = params.NewGlobalParamsCatalog(c.symbols, params.GlobalParamsCatalogConfig{Budget: c.budget})
	if err != nil {
		return nil, err
	}
	c.globals, err = params.NewGlobalParamsService(c.symbols, c.parameters, params.GlobalParamsConfig{Budget: c.budget, Catalog: c.catalog})
	if err != nil {
		return nil, err
	}
	if err = c.globals.InitializeSource(); err != nil {
		return nil, err
	}
	c.cooperation, err = cooperate.NewCooperation(c.budget)
	if err != nil {
		return nil, err
	}
	c.manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: c.symbols, Numerals: c.numerals, Budget: c.budget, ProofMode: ast.ASTProofEnabled})
	if err != nil {
		return nil, err
	}
	if err = c.manager.RegisterDeclarationPlugins(); err != nil {
		return nil, err
	}
	c.terms, err = interp.NewManagerWithParams(c.manager, c.parameters, c.globals)
	if err != nil {
		return nil, err
	}
	c.profiler = interp.ProcessProfiler()
	c.evaluator = model.ModelEvaluatorConfig{Context: c.parameters, Globals: c.globals, Memory: c.numerals.RuntimeOwner(), Cooperation: c.cooperation}
	c.factory, err = smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: c.parameters, Globals: c.globals, ModelEvaluator: c.evaluator})
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *InterpolationContext) open() error {
	if c == nil || c.interpolationState == nil || c.closed {
		return ast.ErrASTHandle
	}
	return nil
}

// Manager borrows the AST owner for term construction and inspection.
func (c *InterpolationContext) Manager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.manager, nil
}

// Terms borrows the typed interpolation term constructors and inspectors.
func (c *InterpolationContext) Terms() (*interp.Manager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.terms, nil
}

// MkInterpolant marks a Boolean expression belonging to this context.
func (c *InterpolationContext) MkInterpolant(expression ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := c.manager.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := c.terms.IsBoolType(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !boolean {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	h, err := c.manager.MkInterp(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := c.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.retained = append(c.retained, ref)
	return h, nil
}

// ComputeInterpolation solves a marked interpolation problem. UNSAT results
// contain borrowed interpolants in marker traversal order. SAT results contain
// an owning model reference when the backend can construct one. UNKNOWN may
// likewise carry a candidate model, matching Z3_compute_interpolant.
func (c *InterpolationContext) ComputeInterpolation(pattern ast.ASTHandle) (result InterpolationResult, resultErr error) {
	if err := c.open(); err != nil {
		return InterpolationResult{Status: lbool.Undef}, err
	}
	var refs []ast.ASTRef
	var mdl *model.ModelCore
	result.Status = lbool.Undef
	// Publish owners only after every temporary owner has closed.
	defer func() {
		if resultErr == nil {
			c.retained = append(c.retained, refs...)
			return
		}
		for i := range refs {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&refs[i]))
		}
		if mdl != nil {
			resultErr = errors.Join(resultErr, mdl.DecRef())
		}
		result = InterpolationResult{Status: lbool.Undef}
	}()
	sort, err := c.manager.GetSort(pattern)
	if err != nil {
		return result, err
	}
	boolean, err := c.terms.IsBoolType(sort)
	if err != nil {
		return result, err
	}
	if !boolean {
		return result, ast.ErrASTDomain
	}
	// Pin the problem through solver teardown, including on translation errors.
	root, err := c.manager.NewRef(pattern)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&root)) }()
	s, err := c.factory.New(c.manager, nil, true, true, true, "")
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.Close()) }()
	config := &interp.RunConfig{Profiler: c.profiler, SolverFactory: c.factory, EvaluatorFactory: func(md *model.ModelCore) (solver.ConsequenceEvaluator, error) {
		return model.NewModelEvaluator(md, c.evaluator)
	}}
	result.Status, _, refs, mdl, err = interp.SolveTree(c.terms, s, pattern, true, config)
	if err != nil {
		return result, err
	}
	result.Model = mdl
	result.Interpolants = make([]ast.ASTHandle, len(refs))
	for i, ref := range refs {
		result.Interpolants[i], err = c.manager.RefGet(ref)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// ComputeInterpolant is the UNSAT-only convenience wrapper. Results follow
// marker traversal order, so And(MkInterpolant(A), B) returns I at index zero
// with A => I and I /\ B unsatisfiable. SAT and UNKNOWN return distinct errors.
func (c *InterpolationContext) ComputeInterpolant(pattern ast.ASTHandle) ([]ast.ASTHandle, error) {
	result, err := c.ComputeInterpolation(pattern)
	if err != nil {
		return nil, err
	}
	if result.Model != nil {
		if err = result.Model.DecRef(); err != nil {
			return nil, err
		}
	}
	if result.Status == lbool.True {
		return nil, ErrInterpolationSAT
	}
	if result.Status != lbool.False {
		return nil, ErrInterpolationUnknown
	}
	return result.Interpolants, nil
}

// InterpolantsFromProof computes interpolants from an existing UNSAT proof and
// marked pattern, matching the retained Z3_get_interpolant proof-input path.
// Returned handles are borrowed from the context until Close.
func (c *InterpolationContext) InterpolantsFromProof(proof, pattern ast.ASTHandle) (result []ast.ASTHandle, resultErr error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	constraints, err := interp.CollectConjuncts(c.terms, pattern)
	if err != nil {
		return nil, err
	}
	config := &interp.RunConfig{Profiler: c.profiler, SolverFactory: c.factory, EvaluatorFactory: func(md *model.ModelCore) (solver.ConsequenceEvaluator, error) {
		return model.NewModelEvaluator(md, c.evaluator)
	}}
	refs, err := interp.TreeInterpolants(c.terms, proof, constraints, pattern, config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr == nil {
			c.retained = append(c.retained, refs...)
			return
		}
		for i := range refs {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&refs[i]))
		}
		result = nil
	}()
	result = make([]ast.ASTHandle, len(refs))
	for i := range refs {
		result[i], err = c.manager.RefGet(refs[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// InterpolationProfile returns the process-lifetime source interpolation timer
// report, including solve and proof phases from other interpolation clients.
func (c *InterpolationContext) InterpolationProfile() (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	var output bytes.Buffer
	if err := c.profiler.Print(&output); err != nil {
		return "", err
	}
	return output.String(), nil
}

// WriteInterpolationSMT2 exports a marked interpolation problem as replayable
// SMT-LIB using ordinary Go output rather than a native filename.
func (c *InterpolationContext) WriteInterpolationSMT2(out io.Writer, pattern ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	constraints, err := interp.CollectConjuncts(c.terms, pattern)
	if err != nil {
		return err
	}
	return interp.PrettyPrintProblem(c.manager, constraints, pattern, out)
}

// InterpolationSMT2 returns the replay stream produced by
// WriteInterpolationSMT2.
func (c *InterpolationContext) InterpolationSMT2(pattern ast.ASTHandle) (string, error) {
	var output strings.Builder
	if err := c.WriteInterpolationSMT2(&output, pattern); err != nil {
		return "", err
	}
	return output.String(), nil
}

// CheckInterpolants validates sequence or tree interpolants and their symbol
// vocabulary. For tree interpolation, parents contains one entry per
// constraint and its final root entry is interp.ScopeTop, matching the source
// API. Diagnostics explains a false result.
func (c *InterpolationContext) CheckInterpolants(constraints []ast.ASTHandle, parents []int, interpolants []ast.ASTHandle, theory []ast.ASTHandle) (valid bool, diagnostics string, resultErr error) {
	if err := c.open(); err != nil {
		return false, "", err
	}
	if len(constraints) == 0 || len(interpolants) != len(constraints)-1 || len(parents) != 0 && len(parents) != len(constraints) {
		return false, "", ast.ErrASTDomain
	}
	s, err := c.factory.New(c.manager, nil, true, false, false, "AUFLIA")
	if err != nil {
		return false, "", err
	}
	defer func() { resultErr = errors.Join(resultErr, s.Close()) }()
	var output bytes.Buffer
	valid, err = interp.CheckInterpolants(c.terms, s, &output, constraints, parents, interpolants, theory)
	return valid, output.String(), err
}

// Close releases results, ASTs and private services in dependency order.
// Caller-owned ASTRefs and solvers must be released before closing the context.
func (c *InterpolationContext) Close() (result error) {
	if c == nil || c.interpolationState == nil {
		return nil
	}
	if c.closed {
		return nil
	}
	c.closed = true
	for i := len(c.retained) - 1; i >= 0; i-- {
		result = errors.Join(result, c.manager.DelRef(&c.retained[i]))
	}
	c.retained = nil
	if c.factory != nil {
		result = errors.Join(result, c.factory.Close())
	}
	if c.terms != nil {
		result = errors.Join(result, c.terms.Close())
	}
	if c.manager != nil {
		result = errors.Join(result, c.manager.Close())
	}
	if c.cooperation != nil {
		result = errors.Join(result, c.cooperation.Close())
	}
	if c.globals != nil {
		result = errors.Join(result, c.globals.Close())
	}
	if c.catalog != nil {
		result = errors.Join(result, c.catalog.Close())
	}
	if c.parameters != nil {
		result = errors.Join(result, c.parameters.Close())
	}
	if c.numerals != nil {
		result = errors.Join(result, c.numerals.Close())
	}
	if c.symbols != nil {
		result = errors.Join(result, c.symbols.Close())
	}
	return result
}
