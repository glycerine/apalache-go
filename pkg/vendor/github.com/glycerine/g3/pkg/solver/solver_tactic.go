package solver

// Source: src/solver/solver2tactic.cpp; src/solver/solver2tactic.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ClauseExtraction owns the references needed between dependency
// normalization and the solver result. Clauses and assumptions are borrowed
// views backed by refs until Close.
type ClauseExtraction struct {
	manager          *ast.ASTManager
	Clauses          []ast.ASTHandle
	Assumptions      []ast.ASTHandle
	BoolToDependency map[ast.ASTHandle]ast.ASTHandle
	Filter           tactic.ModelConverter
	refs             []ast.ASTRef
	closed           bool
}

func (e *ClauseExtraction) pin(h ast.ASTHandle) error {
	r, err := e.manager.NewRef(h)
	if err != nil {
		return err
	}
	e.refs = append(e.refs, r)
	return nil
}

func (e *ClauseExtraction) appendClause(h ast.ASTHandle) error {
	if err := e.pin(h); err != nil {
		return err
	}
	e.Clauses = append(e.Clauses, h)
	return nil
}

func (e *ClauseExtraction) addAssumption(key, dependency ast.ASTHandle) error {
	if _, found := e.BoolToDependency[key]; found {
		return nil
	}
	if err := e.pin(key); err != nil {
		return err
	}
	if dependency != key {
		if err := e.pin(dependency); err != nil {
			return err
		}
	}
	e.Assumptions = append(e.Assumptions, key)
	e.BoolToDependency[key] = dependency
	return nil
}

func (e *ClauseExtraction) ensureFilter() (*tacticFilterInserter, error) {
	if e.Filter != nil {
		filter, ok := e.Filter.(interface{ Insert(ast.ASTHandle) error })
		if !ok {
			return nil, ast.ErrASTHandle
		}
		return &tacticFilterInserter{filter: filter}, nil
	}
	c, err := tactic.NewFilterModel(e.manager)
	if err != nil {
		return nil, err
	}
	if err = c.IncRef(); err != nil {
		_ = c.Close()
		return nil, err
	}
	e.Filter = c
	return &tacticFilterInserter{filter: c}, nil
}

// tacticFilterInserter keeps the concrete Insert surface local while the
// extraction publishes only the general model-converter contract.
type tacticFilterInserter struct {
	filter interface{ Insert(ast.ASTHandle) error }
}

func (e *ClauseExtraction) Close() (err error) {
	if e == nil || e.closed {
		return nil
	}
	if e.Filter != nil {
		err = errors.Join(err, e.Filter.DecRef())
		e.Filter = nil
	}
	for len(e.refs) != 0 {
		err = errors.Join(err, e.manager.DelRef(&e.refs[0]))
		copy(e.refs, e.refs[1:])
		e.refs = e.refs[:len(e.refs)-1]
		if err != nil {
			return err
		}
	}
	e.Clauses = nil
	e.Assumptions = nil
	e.BoolToDependency = nil
	e.closed = true
	return nil
}

// ExtractClausesAndDependencies implements the source solver2tactic helper.
// Boolean constant dependencies are used directly. Other expressions receive
// a fresh Boolean proxy that is removed from produced models.
func ExtractClausesAndDependencies(goal *tactic.Goal) (result *ClauseExtraction, err error) {
	if goal == nil {
		return nil, ast.ErrASTHandle
	}
	if err = goal.CheckOwner(); err != nil {
		return nil, err
	}
	m := goal.Manager()
	result = &ClauseExtraction{manager: m, BoolToDependency: make(map[ast.ASTHandle]ast.ASTHandle)}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	cores, err := goal.UnsatCoresEnabled()
	if err != nil {
		return nil, err
	}
	n, err := goal.Size()
	if err != nil {
		return nil, err
	}
	depToBool := make(map[ast.ASTHandle]ast.ASTHandle)
	for i := uint32(0); i < n; i++ {
		formula, e := goal.Form(i)
		if e != nil {
			return nil, e
		}
		dependency, e := goal.Dependency(i)
		if e != nil {
			return nil, e
		}
		if !cores || dependency.IsNull() {
			if e = result.appendClause(formula); e != nil {
				return nil, e
			}
			continue
		}
		leaves, e := m.DependencyLinearize(dependency, nil)
		if e != nil {
			return nil, e
		}
		if len(leaves) == 0 {
			return nil, ast.ErrASTDomain
		}
		clause := make([]ast.ASTHandle, 1, len(leaves)+1)
		clause[0] = formula
		for _, dep := range leaves {
			b := depToBool[dep]
			if b.IsNull() {
				constant, e := isUninterpretedConstant(m, dep)
				if e != nil {
					return nil, e
				}
				boolean, e := isBoolean(m, dep)
				if e != nil {
					return nil, e
				}
				if constant && boolean {
					b = dep
				} else {
					sort, e := m.MkBoolSort()
					if e != nil {
						return nil, e
					}
					b, e = m.MkFreshConst("", sort)
					if e != nil {
						return nil, e
					}
					depToBool[dep] = b
					inserter, e := result.ensureFilter()
					if e != nil {
						return nil, e
					}
					decl, e := m.GetDecl(b)
					if e != nil {
						return nil, e
					}
					if inserter.filter == nil {
						return nil, ast.ErrASTHandle
					}
					if e = inserter.filter.Insert(decl); e != nil {
						return nil, e
					}
				}
				if e = result.addAssumption(b, dep); e != nil {
					return nil, e
				}
			}
			notB, e := m.MkNot(b)
			if e != nil {
				return nil, e
			}
			clause = append(clause, notB)
		}
		joined, e := m.MkOrArray(clause)
		if e != nil {
			return nil, e
		}
		if e = result.appendClause(joined); e != nil {
			return nil, e
		}
	}
	return result, nil
}

type SolverTacticConfig struct {
	Params                       *uparams.ParamsContext
	TargetConfig                 func(*ast.ASTManager) (SolverTacticConfig, error)
	CandidateModels              bool
	FailIfInconclusive           bool
	InconclusivePolicyConfigured bool
}

func (c SolverTacticConfig) failIfInconclusive() bool {
	if !c.InconclusivePolicyConfigured {
		return true
	}
	return c.FailIfInconclusive
}

type SolverTactic struct {
	self       *SolverTactic
	manager    *ast.ASTManager
	solver     Solver
	config     SolverTacticConfig
	params     uparams.ParamsRef
	stats      statistics.Statistics
	references uint32
	ownerBytes uint64
	closed     bool
}

func NewSolverTactic(s Solver, config SolverTacticConfig) (*SolverTactic, error) {
	if s == nil || config.Params == nil {
		return nil, ast.ErrASTHandle
	}
	m, err := s.Manager()
	if err != nil {
		return nil, err
	}
	if err = config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(SolverTactic{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	t := &SolverTactic{manager: m, solver: s, config: config, params: config.Params.EmptyValue(), ownerBytes: amount}
	t.self = t
	if err = s.IncRef(); err != nil {
		m.Budget().Release(amount)
		return nil, err
	}
	return t, nil
}

func (t *SolverTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.config.Params.Check(&t.params)
}
func (t *SolverTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.references++
	return nil
}
func (t *SolverTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.references == 0 {
		return arena.ErrBounds
	}
	t.references--
	if t.references == 0 {
		return t.Close()
	}
	return nil
}

func closeUnownedModelConverter(c tactic.ModelConverter) error {
	if c == nil {
		return nil
	}
	return c.Close()
}

func (t *SolverTactic) applySAT(in *tactic.Goal, result *tactic.TacticResult, local Solver, extraction *ClauseExtraction) error {
	models, err := in.ModelsEnabled()
	if err != nil {
		return err
	}
	if models {
		md, e := local.AcquireModel()
		if e != nil {
			return e
		}
		stored, e := tactic.NewStoredModel(md)
		if md != nil {
			e = errors.Join(e, md.DecRef())
		}
		if e != nil {
			_ = closeUnownedModelConverter(stored)
			return e
		}
		combined, e := tactic.ConcatModels(t.manager.Budget(), extraction.Filter, stored)
		if e != nil {
			_ = closeUnownedModelConverter(stored)
			return e
		}
		if e = result.SetModelConverter(combined); e != nil {
			if combined != extraction.Filter {
				_ = closeUnownedModelConverter(combined)
			}
			return e
		}
	}
	if err = in.Reset(); err != nil {
		return err
	}
	return result.AddGoal(in)
}

func (t *SolverTactic) applyUNSAT(in *tactic.Goal, result *tactic.TacticResult, local Solver, extraction *ClauseExtraction) error {
	proofs, err := in.ProofsEnabled()
	if err != nil {
		return err
	}
	cores, err := in.UnsatCoresEnabled()
	if err != nil {
		return err
	}
	var proof ast.ASTHandle
	if proofs {
		proof, err = local.Proof()
		if err != nil {
			return err
		}
		converter, e := tactic.NewStoredProofConverter(t.manager, proof)
		if e != nil {
			return e
		}
		if e = result.SetProofConverter(converter); e != nil {
			if converter != nil {
				_ = converter.Close()
			}
			return e
		}
	}
	var dependency ast.ASTDependency
	if cores {
		values, e := local.UnsatCore()
		if e != nil {
			return e
		}
		for _, value := range values {
			original, found := extraction.BoolToDependency[value]
			if !found {
				return ast.ErrASTDomain
			}
			leaf, e := t.manager.DependencyLeaf(original)
			if e != nil {
				return e
			}
			dependency, e = t.manager.DependencyJoin(dependency, leaf)
			if e != nil {
				return e
			}
		}
	}
	if err = in.Reset(); err != nil {
		return err
	}
	falsehood, err := t.manager.MkFalse()
	if err != nil {
		return err
	}
	if err = in.AssertWithProof(falsehood, proof, dependency); err != nil {
		return err
	}
	if err = result.AddGoal(in); err != nil {
		return err
	}
	return result.SetCore(dependency)
}

func (t *SolverTactic) applyCandidateModel(in *tactic.Goal, result *tactic.TacticResult, local Solver) error {
	provider, ok := local.(CandidateModelProvider)
	if !ok {
		return ast.ErrASTDomain
	}
	available, err := provider.PrepareCandidateModel()
	if err != nil {
		return err
	}
	if !available {
		return ast.ErrASTDomain
	}
	models, err := in.ModelsEnabled()
	if err != nil {
		return err
	}
	if models {
		md, e := local.AcquireModel()
		if e != nil {
			return e
		}
		stored, e := tactic.NewStoredModel(md)
		if md != nil {
			e = errors.Join(e, md.DecRef())
		}
		if e != nil {
			_ = closeUnownedModelConverter(stored)
			return e
		}
		if e = result.SetModelConverter(stored); e != nil {
			_ = closeUnownedModelConverter(stored)
			return e
		}
	}
	return result.AddGoal(in)
}

func (t *SolverTactic) Apply(in *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if in == nil || result == nil || in.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	extraction, err := ExtractClausesAndDependencies(in)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, extraction.Close()) }()
	local, err := t.solver.Translate(t.manager, &t.params)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, local.Close()) }()
	for _, clause := range extraction.Clauses {
		if err = local.Assert(clause); err != nil {
			return err
		}
	}
	status, err := local.CheckSat(extraction.Assumptions)
	if err != nil {
		return err
	}
	switch status {
	case lbool.True:
		if t.config.failIfInconclusive() && !in.SatPreserved() {
			return tactic.NewTacticException("over-approximated goal found to be sat")
		}
		err = t.applySAT(in, result, local, extraction)
	case lbool.False:
		if t.config.failIfInconclusive() && !in.UnsatPreserved() {
			return tactic.NewTacticException("under-approximated goal found to be unsat")
		}
		err = t.applyUNSAT(in, result, local, extraction)
	default:
		canceled, e := t.manager.Canceled()
		if e != nil {
			return e
		}
		if canceled {
			return tactic.NewTacticException(messages.Canceled)
		}
		reason, e := local.ReasonUnknown()
		if e != nil {
			return e
		}
		if t.config.CandidateModels {
			if e = t.applyCandidateModel(in, result, local); e == nil {
				break
			} else if !errors.Is(e, ast.ErrASTDomain) {
				return e
			}
		}
		if t.config.failIfInconclusive() && !t.config.CandidateModels {
			return tactic.NewTacticException("smt tactic failed to show goal to be sat/unsat " + reason)
		}
		return tactic.NewTacticException(reason)
	}
	if err != nil {
		return err
	}
	var current statistics.Statistics
	if err = local.CollectStatistics(&current); err != nil {
		return err
	}
	t.stats.Copy(current)
	return nil
}

func (t *SolverTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		return uparams.ErrParamsOwner
	}
	candidateName := "candidate_models"
	candidate, err := t.config.Params.GetBoolName(p, &candidateName, false)
	if err != nil {
		return err
	}
	failName := "fail_if_inconclusive"
	fail, err := t.config.Params.GetBoolName(p, &failName, true)
	if err != nil {
		return err
	}
	if err := t.config.Params.Append(&t.params, p); err != nil {
		return err
	}
	t.config.CandidateModels = candidate
	t.config.FailIfInconclusive = fail
	t.config.InconclusivePolicyConfigured = true
	return t.solver.UpdateParams(p)
}
func (t *SolverTactic) CollectParamDescrs(p *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		return uparams.ErrParamDescrsPrecondition
	}
	candidateDescription := "(default: false) create candidate models even when quantifier or theory reasoning is incomplete."
	candidateDefault := "false"
	if err := p.InsertName("candidate_models", uparams.ParamBool, &candidateDescription, &candidateDefault, nil); err != nil {
		return err
	}
	failDescription := "(default: true) fail if found unsat (sat) for under (over) approximated goal."
	failDefault := "true"
	if err := p.InsertName("fail_if_inconclusive", uparams.ParamBool, &failDescription, &failDefault, nil); err != nil {
		return err
	}
	return t.solver.CollectParamDescrs(p)
}
func (t *SolverTactic) CollectStatistics() map[string]float64 {
	if t == nil || t.open() != nil {
		return nil
	}
	result := make(map[string]float64, t.stats.Size())
	for i := 0; i < t.stats.Size(); i++ {
		key := string(t.stats.Key(i))
		if t.stats.IsUint(i) {
			result[key] += float64(t.stats.UintValue(i))
		} else {
			result[key] += t.stats.DoubleValue(i)
		}
	}
	return result
}
func (t *SolverTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.stats.Reset()
	return nil
}
func (t *SolverTactic) Cleanup() error { return t.open() }
func (t *SolverTactic) Reset() error   { return t.Cleanup() }
func (t *SolverTactic) SetLogic(usymbol.Symbol) error {
	return t.open()
}

type solverTacticProgress struct{ callback func(uint32) }

func (p solverTacticProgress) FastProgressSample() { p.callback(0) }
func (p solverTacticProgress) SlowProgressSample() { p.callback(1) }

func (t *SolverTactic) SetProgressCallback(callback func(uint32)) error {
	if err := t.open(); err != nil {
		return err
	}
	if callback == nil {
		return t.solver.SetProgressCallback(nil)
	}
	return t.solver.SetProgressCallback(solverTacticProgress{callback: callback})
}
func (t *SolverTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	config := t.config
	params := &t.params
	if err := config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		if config.TargetConfig == nil {
			return nil, err
		}
		var e error
		targetConfig, e := config.TargetConfig(m)
		if e != nil {
			return nil, e
		}
		targetConfig.CandidateModels = config.CandidateModels
		targetConfig.FailIfInconclusive = config.FailIfInconclusive
		targetConfig.InconclusivePolicyConfigured = config.InconclusivePolicyConfigured
		config = targetConfig
		params = nil
	}
	translated, err := t.solver.Translate(m, params)
	if err != nil {
		return nil, err
	}
	next, err := NewSolverTactic(translated, config)
	if err != nil {
		_ = translated.Close()
		return nil, err
	}
	return next, nil
}
func (t *SolverTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.config.Params.Del(&t.params); err != nil {
		return err
	}
	if err := t.solver.DecRef(); err != nil {
		return err
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.solver = nil
	t.closed = true
	return nil
}

var _ tactic.Tactic = (*SolverTactic)(nil)
var _ io.Closer = (*SolverTactic)(nil)
