package smtcore

// Source: src/smt/tactic/smt_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
	"github.com/glycerine/g3/pkg/util/messages"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// SMTTactic creates a fresh SMT context per application. Unlike the general
// SolverTactic, it supplies the goal's existing proofs to that context.
type SMTTactic struct {
	self                                *SMTTactic
	manager                             *ast.ASTManager
	config                              SMTSolverConfig
	params                              uparams.ParamsRef
	fullParams                          *SMTParams
	ownedGlobals                        *uparams.GlobalParamsService
	ownedCatalog                        *uparams.GlobalParamsCatalog
	logic                               usymbol.Symbol
	callback                            func(uint32)
	active                              *SMTSolver
	stats                               statistics.Statistics
	candidateModels, failIfInconclusive bool
	refs                                uint32
	ownerBytes                          uint64
	closed                              bool
}

var _ tactic.Tactic = (*SMTTactic)(nil)

func NewSMTTactic(m *ast.ASTManager, p *uparams.ParamsRef, config SMTSolverConfig) (*SMTTactic, error) {
	if m == nil || config.Params == nil {
		return nil, ast.ErrASTHandle
	}
	if err := config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	params, err := config.Params.NewValue()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(SMTTactic{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, errors.Join(err, config.Params.Del(&params))
	}
	t := &SMTTactic{manager: m, config: config, params: params, ownerBytes: amount, failIfInconclusive: true}
	t.self = t
	globals, catalog, err := smtSolverGlobals(m, config)
	if catalog != nil {
		t.ownedGlobals, t.ownedCatalog = globals, catalog
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	t.config.Globals = globals
	empty := config.Params.EmptyValue()
	t.fullParams, err = NewSMTParams(config.Params, &empty, globals, m.Budget())
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	if p != nil {
		if err = config.Params.Assign(&t.params, p); err != nil {
			return nil, errors.Join(err, t.Close())
		}
		if err = t.updatePolicy(p); err != nil {
			return nil, errors.Join(err, t.Close())
		}
	}
	if err = t.traceParams("p: ", "smt_tactic", 53, &t.params); err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func (t *SMTTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.config.Params.Check(&t.params)
}
func (t *SMTTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *SMTTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}

type smtTacticProgress struct{ callback func(uint32) }

func (p smtTacticProgress) FastProgressSample() { p.callback(0) }
func (p smtTacticProgress) SlowProgressSample() { p.callback(1) }

func (t *SMTTactic) Apply(in *tactic.Goal, result *tactic.TacticResult) (err error) {
	defer func() {
		if rewriter.IsRewriterException(err) {
			err = tactic.NewTacticException(err.Error())
		}
	}()
	if err = t.open(); err != nil {
		return err
	}
	if in == nil || result == nil || in.Manager() != t.manager || t.active != nil {
		return ast.ErrASTHandle
	}
	verbose, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil {
		return err
	}
	if verbose {
		stream, e := diagnostic.BorrowProcessVerboseStream()
		if e != nil {
			return e
		}
		if e = stream.AppendCString("(smt.tactic start)\n"); e != nil {
			return e
		}
		if e = stream.Flush(); e != nil {
			return e
		}
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	assertions, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil {
		return err
	}
	if assertions {
		if sorted, e := in.IsWellSorted(); e != nil {
			return e
		} else if !sorted {
			return ast.ErrASTDomain
		}
	}
	if err = t.traceApplyState(in); err != nil {
		return err
	}
	logic := ""
	if t.logic != (usymbol.Symbol{}) {
		if logic, err = t.manager.Symbols.String(t.logic); err != nil {
			return err
		}
	}
	local, err := newSMTSolver(t.manager, &t.params, logic, t.config, t.fullParams)
	if err != nil {
		return err
	}
	if err = t.traceLogic(); err != nil {
		return errors.Join(err, local.Close())
	}
	t.active = local
	defer func() { t.active = nil; err = errors.Join(err, local.Close()) }()
	if err = local.SetProduceModels(local.kernel.smtState.Model); err != nil {
		return err
	}
	if t.callback != nil {
		if err = local.SetProgressCallback(smtTacticProgress{t.callback}); err != nil {
			return err
		}
	}
	cores, err := in.UnsatCoresEnabled()
	if err != nil {
		return err
	}
	proofs, err := in.ProofsEnabled()
	if err != nil {
		return err
	}
	var extraction *solver.ClauseExtraction
	if cores {
		extraction, err = solver.ExtractClausesAndDependencies(in)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, extraction.Close()) }()
		if err = t.traceMUS(in, extraction); err != nil {
			return err
		}
		if proofs && len(extraction.Assumptions) != 0 {
			return tactic.NewTacticException("smt tactic does not support simultaneous generation of proofs and unsat cores")
		}
		for _, clause := range extraction.Clauses {
			if err = local.Assert(clause); err != nil {
				return err
			}
		}
	} else {
		size, e := in.Size()
		if e != nil {
			return e
		}
		for i := uint32(0); i < size; i++ {
			formula, e := in.Form(i)
			if e != nil {
				return e
			}
			var proof ast.ASTHandle
			if proofs {
				if proof, e = in.Proof(i); e != nil {
					return e
				}
			}
			if err = local.kernel.Assert(formula, proof); err != nil {
				return err
			}
		}
	}
	if canceled, e := local.kernel.Canceled(); e != nil {
		return e
	} else if canceled {
		return tactic.NewTacticException(messages.Canceled)
	}
	var assumptions []ast.ASTHandle
	if extraction != nil {
		assumptions = extraction.Assumptions
	}
	var status lbool.LBool
	var checkErr error
	if len(assumptions) == 0 {
		status, checkErr = local.checkSatCore(nil, true)
	} else {
		status, checkErr = local.CheckSat(assumptions)
	}
	// Collect before approximation and UNKNOWN policy, including on a check
	// exception, while the temporary context is still alive.
	statsErr := local.CollectStatistics(&t.stats)
	if err = errors.Join(checkErr, statsErr); err != nil {
		return err
	}
	switch status {
	case lbool.True:
		if t.failIfInconclusive && !in.SatPreserved() {
			return tactic.NewTacticException("over-approximated goal found to be sat")
		}
		if err = in.Reset(); err != nil {
			return err
		}
		if err = result.AddGoal(in); err != nil {
			return err
		}
		var filter tactic.ModelConverter
		if extraction != nil {
			filter = extraction.Filter
		}
		return t.storeModel(in, result, local, filter)
	case lbool.False:
		if t.failIfInconclusive && !in.UnsatPreserved() {
			if err = t.traceApproximationFailure(); err != nil {
				return err
			}
			return tactic.NewTacticException("under-approximated goal found to be unsat")
		}
		return t.storeUnsat(in, result, local, extraction, proofs, cores)
	default:
		if canceled, e := local.kernel.Canceled(); e != nil {
			return e
		} else if canceled {
			return tactic.NewTacticException(messages.Canceled)
		}
		reason, e := local.ReasonUnknown()
		if e != nil {
			return e
		}
		if t.failIfInconclusive && !t.candidateModels {
			return tactic.NewTacticException("smt tactic failed to show goal to be sat/unsat " + reason)
		}
		if err = result.AddGoal(in); err != nil {
			return err
		}
		if t.candidateModels {
			available, e := local.PrepareCandidateModel()
			if e != nil {
				return e
			}
			if available {
				return t.storeModel(in, result, local, nil)
			}
		}
		return tactic.NewTacticException(reason)
	}
}

func (t *SMTTactic) storeModel(in *tactic.Goal, result *tactic.TacticResult, local *SMTSolver, filter tactic.ModelConverter) (err error) {
	models, err := in.ModelsEnabled()
	if err != nil || !models {
		return err
	}
	md, err := local.AcquireModel()
	if err != nil {
		return err
	}
	if md != nil {
		defer func() { err = errors.Join(err, md.DecRef()) }()
	}
	labels, err := local.Labels()
	if err != nil {
		return err
	}
	stored, err := tactic.NewStoredModelWithLabels(md, labels)
	if err != nil {
		return err
	}
	combined, err := tactic.ConcatModels(t.manager.Budget(), filter, stored)
	if err != nil {
		if stored != nil {
			err = errors.Join(err, stored.Close())
		}
		return err
	}
	if err = result.SetModelConverter(combined); err != nil && combined != nil && combined != filter {
		err = errors.Join(err, combined.Close())
	}
	return err
}

func (t *SMTTactic) storeUnsat(in *tactic.Goal, result *tactic.TacticResult, local *SMTSolver, extraction *solver.ClauseExtraction, proofs, cores bool) error {
	var proof ast.ASTHandle
	var err error
	if proofs {
		if proof, err = local.Proof(); err != nil {
			return err
		}
	}
	var dependency ast.ASTDependency
	if cores {
		core, e := local.UnsatCore()
		if e != nil {
			return e
		}
		for _, b := range core {
			original, found := extraction.BoolToDependency[b]
			if !found {
				return ast.ErrASTDomain
			}
			leaf, e := t.manager.DependencyLeaf(original)
			if e != nil {
				return e
			}
			if dependency, e = t.manager.DependencyJoin(dependency, leaf); e != nil {
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
	// Native keeps proof and dependency on the false goal entry, leaving the
	// separate proof-converter and core outputs empty.
	return result.AddGoal(in)
}

func (t *SMTTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		return uparams.ErrParamsOwner
	}
	if err := t.traceParams("updt_params: ", "updt_params", 78, p); err != nil {
		return err
	}
	if err := t.updatePolicy(p); err != nil {
		return err
	}
	if err := t.fullParams.Update(p); err != nil {
		return err
	}
	if err := t.config.Params.Copy(&t.params, p); err != nil {
		return err
	}
	logicName := "logic"
	logic, err := t.config.Params.GetSymbolName(p, &logicName, t.logic)
	if err != nil {
		return err
	}
	t.logic = logic
	return nil
}

func (t *SMTTactic) updatePolicy(p *uparams.ParamsRef) error {
	c := t.config.Params
	candidateName, failName := "candidate_models", "fail_if_inconclusive"
	candidate, err := c.GetBoolName(p, &candidateName, false)
	if err != nil {
		return err
	}
	fail, err := c.GetBoolName(p, &failName, true)
	if err != nil {
		return err
	}
	t.candidateModels, t.failIfInconclusive = candidate, fail
	return nil
}

func (t *SMTTactic) CollectParamDescrs(p *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		return uparams.ErrParamDescrsPrecondition
	}
	candidate := "(default: false) create candidate models even when quantifier or theory reasoning is incomplete."
	fail := "(default: true) fail if found unsat (sat) for under (over) approximated goal."
	if err := p.InsertName("candidate_models", uparams.ParamBool, &candidate, nil, nil); err != nil {
		return err
	}
	if err := p.InsertName("fail_if_inconclusive", uparams.ParamBool, &fail, nil, nil); err != nil {
		return err
	}
	if err := catalogCollectSmtParamsHelper(p); err != nil {
		return err
	}
	return lp.CollectParamDescrs(p)
}

func (t *SMTTactic) CollectStatistics() map[string]float64 {
	if t == nil || t.open() != nil {
		return nil
	}
	var stats statistics.Statistics
	if t.active != nil {
		if t.active.CollectStatistics(&stats) != nil {
			return nil
		}
	} else {
		stats.Copy(t.stats)
	}
	result := make(map[string]float64, stats.Size())
	for i := 0; i < stats.Size(); i++ {
		key := string(stats.Key(i))
		if stats.IsUint(i) {
			result[key] += float64(stats.UintValue(i))
		} else {
			result[key] += stats.DoubleValue(i)
		}
	}
	return result
}
func (t *SMTTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.stats.Reset()
	return nil
}
func (t *SMTTactic) Cleanup() error { return t.open() }
func (t *SMTTactic) Reset() error   { return t.Cleanup() }
func (t *SMTTactic) SetLogic(logic usymbol.Symbol) error {
	if err := t.open(); err != nil {
		return err
	}
	t.logic = logic
	return nil
}
func (t *SMTTactic) SetProgressCallback(callback func(uint32)) error {
	if err := t.open(); err != nil {
		return err
	}
	t.callback = callback
	if t.active != nil {
		if callback == nil {
			return t.active.SetProgressCallback(nil)
		}
		return t.active.SetProgressCallback(smtTacticProgress{callback})
	}
	return nil
}
func (t *SMTTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	// Retained managers share process-owned symbols and numerals. The
	// constructor verifies these services before copying the parameter owner.
	config := t.config
	if t.ownedCatalog != nil {
		// A translated tactic must not borrow a service that closing its
		// source will destroy.
		config.Globals = nil
	}
	return NewSMTTactic(m, &t.params, config)
}
func (t *SMTTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.active != nil {
		return ast.ErrASTDomain
	}
	if t.fullParams != nil {
		if err := t.fullParams.Close(); err != nil {
			return err
		}
		t.fullParams = nil
	}
	if err := t.config.Params.Del(&t.params); err != nil {
		return err
	}
	if t.ownedGlobals != nil {
		if err := t.ownedGlobals.Close(); err != nil {
			return err
		}
		t.ownedGlobals = nil
	}
	if t.ownedCatalog != nil {
		if err := t.ownedCatalog.Close(); err != nil {
			return err
		}
		t.ownedCatalog = nil
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
