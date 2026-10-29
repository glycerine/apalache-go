package spacer

// Source: src/muz/spacer/spacer_itp_solver.h and
// src/muz/spacer/spacer_itp_solver.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrITPSolverOwner = errors.New("muz/spacer: invalid or closed interpolating solver")
var ErrITPCoreMode = errors.New("muz/spacer: unsupported interpolating-core mode")

type ItpCoreMode uint8

const (
	ItpCoreUnspecified ItpCoreMode = iota
	ItpCoreLegacyFarkas
	ItpCoreNewFarkas
	ItpCoreNewOptimized
)

type ItpSolverConfig struct {
	Params            *uparams.ParamsContext
	Globals           *uparams.GlobalParamsService
	SplitLiterals     bool
	CoreMode          ItpCoreMode
	FarkasAConst      bool
	MinimizeUnsatCore bool
}

type itpDefinitions struct {
	expressionToProxy map[ast.ASTHandle]ast.ASTHandle
	proxyToDefinition map[ast.ASTHandle]ast.ASTHandle
	references        []ast.ASTRef
}

func newItpDefinitions() itpDefinitions {
	return itpDefinitions{
		expressionToProxy: make(map[ast.ASTHandle]ast.ASTHandle),
		proxyToDefinition: make(map[ast.ASTHandle]ast.ASTHandle),
	}
}

func (d *itpDefinitions) close(manager *ast.ASTManager) (result error) {
	for i := len(d.references) - 1; i >= 0; i-- {
		result = errors.Join(result, manager.DelRef(&d.references[i]))
	}
	d.references = nil
	d.expressionToProxy = nil
	d.proxyToDefinition = nil
	return result
}

// ItpSolver adds the Spacer assumption-proxy and interpolating-core protocol
// to a borrowed incremental solver. Close releases wrapper-owned AST state; it
// does not close the backend.
type ItpSolver struct {
	manager         *ast.ASTManager
	backend         solver.Solver
	config          ItpSolverConfig
	proxies         []ast.ASTRef
	activeProxies   uint32
	baseDefinitions itpDefinitions
	scopes          []itpDefinitions
	assumptions     []ast.ASTRef
	firstAssumption uint32
	proxied         bool
	closed          bool
}

func NewItpSolver(backend solver.Solver, config ItpSolverConfig) (*ItpSolver, error) {
	if backend == nil || config.Params == nil || config.Globals == nil || config.Globals.Context != config.Params {
		return nil, ErrITPSolverOwner
	}
	if config.CoreMode == ItpCoreUnspecified {
		config.CoreMode = ItpCoreNewOptimized
	}
	if config.CoreMode != ItpCoreLegacyFarkas && config.CoreMode != ItpCoreNewFarkas && config.CoreMode != ItpCoreNewOptimized {
		return nil, ErrITPCoreMode
	}
	manager, err := backend.Manager()
	if err != nil {
		return nil, err
	}
	if manager == nil || config.Params.Symbols() != manager.Symbols {
		return nil, ErrITPSolverOwner
	}
	return &ItpSolver{manager: manager, backend: backend, config: config, baseDefinitions: newItpDefinitions()}, nil
}

func (s *ItpSolver) open() error {
	if s == nil || s.closed || s.manager == nil || s.backend == nil {
		return ErrITPSolverOwner
	}
	return nil
}

func (s *ItpSolver) definitions() *itpDefinitions {
	if len(s.scopes) != 0 {
		return &s.scopes[len(s.scopes)-1]
	}
	return &s.baseDefinitions
}

func (s *ItpSolver) freshProxy() (ast.ASTHandle, error) {
	if s.activeProxies < uint32(len(s.proxies)) {
		result, err := s.manager.RefGet(s.proxies[s.activeProxies])
		if err == nil {
			s.activeProxies++
		}
		return result, err
	}
	boolean, err := s.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	proxy, err := s.manager.MkFreshConst(fmt.Sprintf("spacer_proxy!%d", len(s.proxies)), boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	reference, err := s.manager.NewRef(proxy)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.proxies = append(s.proxies, reference)
	s.activeProxies++
	return proxy, nil
}

func (s *ItpSolver) directLiteral(expression ast.ASTHandle) (bool, error) {
	if negated, argument, err := s.manager.IsNot(expression); err != nil {
		return false, err
	} else if negated {
		expression = argument
	}
	kind, err := s.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	declaration, err := s.manager.GetDecl(expression)
	if err != nil {
		return false, err
	}
	family, err := s.manager.GetFamilyID(declaration)
	if err != nil || family != ast.NullFamilyID {
		return false, err
	}
	arity, err := s.manager.GetArity(declaration)
	return arity == 0, err
}

func (s *ItpSolver) makeProxy(expression ast.ASTHandle) (_ ast.ASTHandle, resultErr error) {
	direct, err := s.directLiteral(expression)
	if err != nil || direct {
		return expression, err
	}
	definitions := s.definitions()
	if proxy, found := definitions.expressionToProxy[expression]; found {
		return proxy, nil
	}
	proxy, err := s.freshProxy()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	committed := false
	defer func() {
		if !committed {
			s.activeProxies--
		}
	}()
	negated, err := s.manager.MkNot(proxy)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	definition, err := s.manager.MkOr(negated, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	reference, err := s.manager.NewRef(definition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	definitions.references = append(definitions.references, reference)
	definitions.expressionToProxy[expression] = proxy
	definitions.proxyToDefinition[proxy] = definition
	if err = s.backend.Assert(definition); err != nil {
		delete(definitions.expressionToProxy, expression)
		delete(definitions.proxyToDefinition, proxy)
		definitions.references = definitions.references[:len(definitions.references)-1]
		resultErr = errors.Join(err, s.manager.DelRef(&reference))
		return ast.ASTHandle{}, resultErr
	}
	committed = true
	return proxy, nil
}

func (s *ItpSolver) proxyDefinition(proxy ast.ASTHandle) (ast.ASTHandle, bool) {
	for i := len(s.scopes) - 1; i >= 0; i-- {
		if definition, found := s.scopes[i].proxyToDefinition[proxy]; found {
			return definition, true
		}
	}
	definition, found := s.baseDefinitions.proxyToDefinition[proxy]
	return definition, found
}

func (s *ItpSolver) trimAssumptions(size uint32) (result error) {
	for uint32(len(s.assumptions)) > size {
		last := len(s.assumptions) - 1
		result = errors.Join(result, s.manager.DelRef(&s.assumptions[last]))
		s.assumptions = s.assumptions[:last]
	}
	return result
}

func (s *ItpSolver) appendAssumption(expression ast.ASTHandle) error {
	reference, err := s.manager.NewRef(expression)
	if err != nil {
		return err
	}
	s.assumptions = append(s.assumptions, reference)
	return nil
}

func (s *ItpSolver) assumptionHandles() ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(s.assumptions))
	for i := range s.assumptions {
		var err error
		result[i], err = s.manager.RefGet(s.assumptions[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *ItpSolver) proxyRange(from uint32) (bool, error) {
	dirty := false
	for i := from; i < uint32(len(s.assumptions)); i++ {
		expression, err := s.manager.RefGet(s.assumptions[i])
		if err != nil {
			return false, err
		}
		proxy, err := s.makeProxy(expression)
		if err != nil {
			return false, err
		}
		if proxy == expression {
			continue
		}
		reference, err := s.manager.NewRef(proxy)
		if err != nil {
			return false, err
		}
		old := s.assumptions[i]
		s.assumptions[i] = reference
		if err = s.manager.DelRef(&old); err != nil {
			return false, err
		}
		dirty = true
	}
	return dirty, nil
}

func (s *ItpSolver) Assert(expression ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.Assert(expression)
}

func (s *ItpSolver) Push() error {
	if err := s.open(); err != nil {
		return err
	}
	if err := s.backend.Push(); err != nil {
		return err
	}
	s.scopes = append(s.scopes, newItpDefinitions())
	return nil
}

func (s *ItpSolver) Pop(count uint32) (result error) {
	if err := s.open(); err != nil {
		return err
	}
	if count > uint32(len(s.scopes)) {
		return ast.ErrASTDomain
	}
	if err := s.backend.Pop(count); err != nil {
		return err
	}
	for count != 0 {
		last := len(s.scopes) - 1
		s.activeProxies -= uint32(len(s.scopes[last].references))
		result = errors.Join(result, s.scopes[last].close(s.manager))
		s.scopes = s.scopes[:last]
		count--
	}
	return result
}

func (s *ItpSolver) PushBackground(expression ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := s.trimAssumptions(s.firstAssumption); err != nil {
		return err
	}
	if err := s.appendAssumption(expression); err != nil {
		return err
	}
	s.firstAssumption = uint32(len(s.assumptions))
	return nil
}

func (s *ItpSolver) PopBackground(count uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if count > s.firstAssumption {
		return ast.ErrASTDomain
	}
	if err := s.trimAssumptions(s.firstAssumption); err != nil {
		return err
	}
	s.firstAssumption -= count
	return s.trimAssumptions(s.firstAssumption)
}

func (s *ItpSolver) NumBackground() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.firstAssumption, nil
}

func (s *ItpSolver) CheckSat(assumptions []ast.ASTHandle) (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	if err := s.trimAssumptions(s.firstAssumption); err != nil {
		return lbool.Undef, err
	}
	if _, err := s.proxyRange(0); err != nil {
		return lbool.Undef, err
	}
	s.firstAssumption = uint32(len(s.assumptions))
	for _, assumption := range assumptions {
		if err := s.appendAssumption(assumption); err != nil {
			return lbool.Undef, err
		}
	}
	proxied, err := s.proxyRange(s.firstAssumption)
	if err != nil {
		return lbool.Undef, err
	}
	s.proxied = proxied
	values, err := s.assumptionHandles()
	if err != nil {
		return lbool.Undef, err
	}
	return s.backend.CheckSat(values)
}

// FullUnsatCore returns the backend core before background filtering and proxy
// expansion. The handles borrow backend/wrapper-owned state.
func (s *ItpSolver) FullUnsatCore() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.backend.UnsatCore()
}

// UnsatCore removes background assumptions and expands check-local proxies.
func (s *ItpSolver) UnsatCore() ([]ast.ASTHandle, error) {
	core, err := s.FullUnsatCore()
	if err != nil {
		return nil, err
	}
	background := make(map[ast.ASTHandle]bool, s.firstAssumption)
	for i := uint32(0); i < s.firstAssumption; i++ {
		handle, err := s.manager.RefGet(s.assumptions[i])
		if err != nil {
			return nil, err
		}
		background[handle] = true
	}
	result := make([]ast.ASTHandle, 0, len(core))
	for _, expression := range core {
		if background[expression] {
			continue
		}
		if s.proxied {
			if definition, found := s.proxyDefinition(expression); found {
				original, err := s.manager.GetArg(definition, 1)
				if err != nil {
					return nil, err
				}
				expression = original
			}
		}
		result = append(result, expression)
	}
	return result, nil
}

func (s *ItpSolver) AcquireModel() (*model.ModelCore, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.backend.AcquireModel()
}

func (s *ItpSolver) SetProduceModels(enabled bool) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.SetProduceModels(enabled)
}

func (s *ItpSolver) eliminateProxies(values []ast.ASTRef) (_ []ast.ASTRef, resultErr error) {
	defer func() {
		for i := range values {
			if values[i] != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, s.manager.DelRef(&values[i]))
			}
		}
	}()
	proofs, err := s.manager.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	substitution, err := s.manager.NewExprSubstitutionPolicy(false, proofs)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelExprSubstitution(&substitution)) }()
	truth, err := s.manager.MkTrue()
	if err != nil {
		return nil, err
	}
	var proof ast.ASTHandle
	if proofs {
		proof, err = s.manager.MkProofAsserted(truth)
		if err != nil {
			return nil, err
		}
	}
	for _, reference := range s.proxies {
		proxy, err := s.manager.RefGet(reference)
		if err != nil {
			return nil, err
		}
		if err = s.manager.ExprSubstitutionInsert(substitution, proxy, truth, proof, ast.ASTDependency{}); err != nil {
			return nil, err
		}
	}
	replacer, err := rewriter.NewDefaultExprReplacer(s.manager)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, replacer.Close()) }()
	if err = replacer.SetSubstitution(&substitution); err != nil {
		return nil, err
	}
	result := make([]ast.ASTRef, 0, len(values))
	committed := false
	defer func() {
		if !committed {
			for i := range result {
				resultErr = errors.Join(resultErr, s.manager.DelRef(&result[i]))
			}
		}
	}()
	for i := range values {
		handle, err := s.manager.RefGet(values[i])
		if err != nil {
			return nil, err
		}
		rewritten, err := s.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, err
		}
		if err = replacer.Rewrite(handle, rewritten); err != nil {
			resultErr = errors.Join(err, s.manager.DelRef(&rewritten))
			return nil, resultErr
		}
		rewrittenHandle, err := s.manager.RefGet(rewritten)
		if err != nil {
			_ = s.manager.DelRef(&rewritten)
			return nil, err
		}
		if !s.manager.IsTrue(rewrittenHandle) {
			result = append(result, rewritten)
		} else if err = s.manager.DelRef(&rewritten); err != nil {
			return nil, err
		}
	}
	committed = true
	return result, nil
}

// ItpCore extracts the retained old-style Spacer Farkas core. Returned
// references are caller-owned.
func (s *ItpSolver) ItpCore() (_ []ast.ASTRef, resultErr error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	proof, err := s.backend.Proof()
	if err != nil {
		return nil, err
	}
	bAssertions := make([]ast.ASTHandle, 0, len(s.assumptions)-int(s.firstAssumption))
	for i := s.firstAssumption; i < uint32(len(s.assumptions)); i++ {
		assumption, err := s.manager.RefGet(s.assumptions[i])
		if err != nil {
			return nil, err
		}
		if definition, found := s.proxyDefinition(assumption); found {
			bAssertions = append(bAssertions, definition)
		}
		bAssertions = append(bAssertions, assumption)
	}
	var values []ast.ASTRef
	if s.config.CoreMode == ItpCoreLegacyFarkas {
		learner, err := NewFarkasLearner(s.manager, s.config.Params, s.config.Globals)
		if err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, learner.Close()) }()
		if err = learner.SetSplitLiterals(s.config.SplitLiterals); err != nil {
			return nil, err
		}
		values, err = learner.GetLemmas(proof, bAssertions)
		if err != nil {
			return nil, err
		}
	} else {
		learner, err := NewUnsatCoreLearner(s.manager)
		if err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, learner.Close()) }()
		if s.config.CoreMode == ItpCoreNewOptimized {
			farkas, err := NewUnsatCoreOptimizedFarkasPlugin(learner, s.config.Params, s.config.Globals)
			if err != nil {
				return nil, err
			}
			if err = learner.RegisterPlugin(farkas); err != nil {
				return nil, errors.Join(err, farkas.Close())
			}
		} else {
			farkas, err := NewUnsatCoreFarkasPlugin(learner, s.config.Params, s.config.Globals, s.config.SplitLiterals, s.config.FarkasAConst)
			if err != nil {
				return nil, err
			}
			if err = learner.RegisterPlugin(farkas); err != nil {
				return nil, errors.Join(err, farkas.Close())
			}
		}
		var fallback UnsatCorePlugin
		if s.config.MinimizeUnsatCore {
			fallback, err = NewUnsatCoreMinCutPlugin(learner)
		} else {
			fallback, err = NewUnsatCoreLemmaPlugin(learner)
		}
		if err != nil {
			return nil, err
		}
		if err = learner.RegisterPlugin(fallback); err != nil {
			return nil, errors.Join(err, fallback.Close())
		}
		values, err = learner.ComputeUnsatCore(proof, bAssertions)
		if err != nil {
			return nil, err
		}
	}
	values, err = s.eliminateProxies(values)
	if err != nil {
		return nil, err
	}
	return simplifyBounds(s.manager, s.config.Params, s.config.Globals, values)
}

func (s *ItpSolver) CollectStatistics(destination *statistics.Statistics) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.CollectStatistics(destination)
}

func (s *ItpSolver) ResetStatistics() error {
	return s.open()
}

func (s *ItpSolver) Close() (result error) {
	if s == nil {
		return ErrITPSolverOwner
	}
	if s.closed {
		return nil
	}
	result = errors.Join(result, s.trimAssumptions(0))
	if len(s.scopes) != 0 {
		result = errors.Join(result, s.backend.Pop(uint32(len(s.scopes))))
	}
	for i := len(s.scopes) - 1; i >= 0; i-- {
		result = errors.Join(result, s.scopes[i].close(s.manager))
	}
	result = errors.Join(result, s.baseDefinitions.close(s.manager))
	for i := len(s.proxies) - 1; i >= 0; i-- {
		result = errors.Join(result, s.manager.DelRef(&s.proxies[i]))
	}
	s.scopes = nil
	s.proxies = nil
	s.backend = nil
	s.manager = nil
	s.closed = true
	return result
}
