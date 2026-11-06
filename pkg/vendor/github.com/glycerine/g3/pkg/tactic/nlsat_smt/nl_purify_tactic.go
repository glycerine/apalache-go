package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp/.h, tactic lifecycle.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type NLPurifyTactic struct {
	self            *NLPurifyTactic
	manager         *ast.ASTManager
	context         *params.ParamsContext
	globals         *params.GlobalParamsService
	services        model.ModelEvaluatorConfig
	parameters      params.ParamsRef
	paramsReady     bool
	smt             *smtcore.SMTSolver
	nonlinear       *nlsattactic.NLSATTactic
	refs            uint32
	ownerBytes      uint64
	running, closed bool
}

var _ tactic.Tactic = (*NLPurifyTactic)(nil)

func NewNLPurifyTactic(m *ast.ASTManager, context *params.ParamsContext, globals *params.GlobalParamsService, local *params.ParamsRef, services model.ModelEvaluatorConfig) (_ *NLPurifyTactic, resultErr error) {
	if m == nil || context == nil || globals == nil || globals.Context != context || services.Context != context || services.Globals != globals || services.Memory == nil || services.Cooperation == nil {
		return nil, params.ErrParamsOwner
	}
	if err := context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(NLPurifyTactic{}).Size())
	if err := m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	t := &NLPurifyTactic{manager: m, context: context, globals: globals, services: services, ownerBytes: amount}
	t.self = t
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, t.Close())
		}
	}()
	var err error
	t.parameters, err = context.NewValue()
	if err != nil {
		return nil, err
	}
	t.paramsReady = true
	if local != nil {
		if err = context.Assign(&t.parameters, local); err != nil {
			return nil, err
		}
	}
	t.nonlinear, err = nlsattactic.NewNLSATTactic(m, context, globals, &t.parameters, services)
	if err != nil {
		return nil, err
	}
	if err = t.makeSolver(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *NLPurifyTactic) makeSolver() error {
	config := smtcore.SMTSolverConfig{Params: t.context, Globals: t.globals, ModelEvaluator: t.services}
	s, err := smtcore.NewSMTSolver(t.manager, &t.parameters, "", config)
	if err != nil {
		return err
	}
	if err = s.SetProduceModels(true); err != nil {
		return errors.Join(err, s.Close())
	}
	t.smt = s
	return nil
}

func (t *NLPurifyTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || !t.paramsReady {
		return arena.ErrClosed
	}
	if t.running {
		return ast.ErrASTPending
	}
	return t.context.Check(&t.parameters)
}
func (t *NLPurifyTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *NLPurifyTactic) DecRef() error {
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
func (t *NLPurifyTactic) Close() (result error) {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.running {
		return ast.ErrASTPending
	}
	if t.smt != nil {
		if err := t.smt.Close(); err != nil {
			return err
		}
		t.smt = nil
	}
	if t.nonlinear != nil {
		if err := t.nonlinear.Close(); err != nil {
			return err
		}
		t.nonlinear = nil
	}
	if t.paramsReady {
		if err := t.context.Del(&t.parameters); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *NLPurifyTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.smt != nil {
		if err := t.smt.Close(); err != nil {
			return err
		}
		t.smt = nil
	}
	if err := t.nonlinear.Cleanup(); err != nil {
		return err
	}
	return t.makeSolver()
}
func (t *NLPurifyTactic) Reset() error { return t.Cleanup() }
func (t *NLPurifyTactic) UpdateParams(p *params.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.context.Assign(&t.parameters, p)
}
func (t *NLPurifyTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewNLPurifyTactic(m, t.context, t.globals, &t.parameters, t.services)
}
func (t *NLPurifyTactic) CollectParamDescrs(*params.ParamDescrs) error { return t.open() }
func (t *NLPurifyTactic) CollectStatistics() map[string]float64 {
	if t == nil || t.closed || t.nonlinear == nil || t.smt == nil {
		return nil
	}
	result := t.nonlinear.CollectStatistics()
	if result == nil {
		result = make(map[string]float64)
	}
	var s statistics.Statistics
	if t.smt.CollectStatistics(&s) != nil {
		return result
	}
	for i := 0; i < s.Size(); i++ {
		value := 0.0
		if s.IsUint(i) {
			value = float64(s.UintValue(i))
		} else {
			value = s.DoubleValue(i)
		}
		result[string(s.Key(i))] += value
	}
	return result
}
func (t *NLPurifyTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.nonlinear.ResetStatistics()
}
func (t *NLPurifyTactic) SetLogic(symbol.Symbol) error           { return t.open() }
func (t *NLPurifyTactic) SetProgressCallback(func(uint32)) error { return t.open() }

func (t *NLPurifyTactic) Apply(goal *tactic.Goal, out *tactic.TacticResult) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if goal == nil || out == nil || goal.Manager() != t.manager || t.smt == nil {
		return ast.ErrASTHandle
	}
	t.running = true
	defer func() { t.running = false }()
	report, err := tactic.NewTacticReport("qfufnl-purify", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { resultErr = errors.Join(resultErr, report.Close()) }()
	}
	if err = emitNLTrace(t.manager, t.services, "operator()", 754, goal.Display); err != nil {
		return err
	}
	prepared, err := prepareNLPurification(goal)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, prepared.Close()) }()
	if err = prepared.assertSMT(t.smt); err != nil {
		return err
	}
	cooperation, err := newNLCooperation(prepared, t.smt, t.nonlinear, t.services)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, cooperation.Close()) }()
	return cooperation.solve(out)
}
