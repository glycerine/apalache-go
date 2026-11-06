package qe

// Source: src/qe/qe_sat_tactic.cpp/.h, tactic lifecycle and goal application.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// QESATTactic supports model-based partial projection and selected scalar
// full projections, with optional strong context simplification.
type QESATTactic struct {
	self                *QESATTactic
	manager             *ast.ASTManager
	config              Config
	params              uparams.ParamsRef
	options             satTacticOptions
	levelStats          statistics.Statistics
	persistentStats     statistics.Statistics
	refs                uint32
	bytes               uint64
	paramsReady, closed bool
}

var _ tactic.Tactic = (*QESATTactic)(nil)

func NewQESATTactic(m *ast.ASTManager, params *uparams.ParamsRef, config Config) (*QESATTactic, error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err := config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	size := uint64(reflect.TypeOf(QESATTactic{}).Size())
	if err := m.Budget().Reserve(size); err != nil {
		return nil, err
	}
	t := &QESATTactic{manager: m, config: config, options: newSATTacticOptions(), bytes: size}
	t.self = t
	var err error
	t.params, err = config.Context.NewValue()
	t.paramsReady = err == nil
	if err == nil && params != nil {
		err = config.Context.Assign(&t.params, params)
		if err == nil {
			err = t.options.update(config.Context, &t.params)
		}
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func (t *QESATTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.config.Context.Check(&t.params)
}
func (t *QESATTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *QESATTactic) DecRef() error {
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
func (t *QESATTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.paramsReady {
		if err := t.config.Context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.bytes)
	t.bytes = 0
	t.closed = true
	return nil
}
func (t *QESATTactic) Cleanup() error { return t.open() }
func (t *QESATTactic) Reset() error   { return t.open() }
func (t *QESATTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewQESATTactic(m, nil, t.config)
}
func (t *QESATTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		return ast.ErrASTHandle
	}
	if err := t.config.Context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.options.update(t.config.Context, p)
}
func (t *QESATTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return collectSATTacticOptions(out)
}
func (t *QESATTactic) CollectStatistics() map[string]float64 {
	if t.open() != nil {
		return nil
	}
	var collected statistics.Statistics
	collected.Copy(t.persistentStats)
	collected.Copy(t.levelStats)
	result := make(map[string]float64, collected.Size())
	for i := 0; i < collected.Size(); i++ {
		key := string(collected.Key(i))
		if collected.IsUint(i) {
			result[key] += float64(collected.UintValue(i))
		} else {
			result[key] += collected.DoubleValue(i)
		}
	}
	return result
}
func (t *QESATTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.levelStats.Reset()
	t.persistentStats.Reset()
	return nil
}
func (t *QESATTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *QESATTactic) SetProgressCallback(func(uint32)) error { return t.open() }

func (t *QESATTactic) Apply(goal *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || goal.Manager() != t.manager || out == nil {
		return ast.ErrASTHandle
	}
	if err = tactic.FailIfProofGeneration("qe-sat", goal); err != nil {
		return err
	}
	if err = tactic.FailIfUnsatCoreGeneration("qe-sat", goal); err != nil {
		return err
	}
	m := t.manager
	canceled, err := m.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(m.ResourceLimit().CancelMessage())
	}
	if err = t.config.Cooperation.Cooperate("qe-sat"); err != nil {
		return err
	}
	// The source tactic resets its per-application level solvers before
	// preparing the next goal. Its auxiliary solver and context rewriter live
	// across applications, so their statistics are accumulated separately.
	t.levelStats.Reset()
	if err = out.Prepare(m); err != nil {
		return err
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	forms := make([]ast.ASTHandle, size)
	for i := range forms {
		forms[i], err = goal.Form(uint32(i))
		if err != nil {
			return err
		}
	}
	conjunction, err := m.MkAndArgs(forms)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(conjunction)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&ref)) }()
	preprocessed, err := simplifyQEExpression(m, conjunction)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&preprocessed)) }()
	conjunction, err = m.RefGet(preprocessed)
	if err != nil {
		return err
	}
	alt, err := extractSATAlternation(m, t.config, conjunction)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, alt.Close()) }()
	search, err := newSATPartialSearch(m, t.config, alt, t.options)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, search.Close()) }()
	defer func() {
		var level, persistent statistics.Statistics
		if collectErr := search.collectStatistics(&level, &persistent); collectErr != nil {
			err = errors.Join(err, collectErr)
			return
		}
		t.levelStats = level
		t.persistentStats.Copy(persistent)
	}()
	truth, err := m.MkTrue()
	if err != nil {
		return err
	}
	result, err := search.qt(0, truth)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, result.Close()) }()
	h, err := m.RefGet(result.formula)
	if err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if m.IsFalse(h) {
		if err = goal.Assert(h); err != nil {
			return err
		}
	} else {
		if result.model == nil {
			return ast.ErrASTDomain
		}
		converter, e := tactic.NewStoredModel(result.model)
		if e != nil {
			return e
		}
		if e = out.SetModelConverter(converter); e != nil {
			return errors.Join(e, converter.Close())
		}
		if err = goal.Reset(); err != nil {
			return err
		}
	}
	return out.AddGoal(goal)
}
