package core

// Source: src/tactic/core/collect_statistics_tactic.cpp/.h.

import (
	"errors"
	"io"
	"os"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type CollectStatisticsTactic struct {
	self                *CollectStatisticsTactic
	manager             *ast.ASTManager
	context             *uparams.ParamsContext
	params              uparams.ParamsRef
	output              io.Writer
	stats               map[string]uint64
	refs                uint32
	ownerBytes          uint64
	paramsReady, closed bool
}

var _ tactic.Tactic = (*CollectStatisticsTactic)(nil)

func NewCollectStatisticsTactic(m *ast.ASTManager, p *uparams.ParamsRef) (*CollectStatisticsTactic, error) {
	return NewCollectStatisticsTacticWithOutput(m, p, os.Stdout)
}

func NewCollectStatisticsTacticWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, output io.Writer) (*CollectStatisticsTactic, error) {
	if m == nil || output == nil {
		return nil, ast.ErrASTHandle
	}
	context, err := collectStatisticsParamsContext(m)
	if err != nil {
		return nil, err
	}
	return newCollectStatisticsTactic(m, context, p, output)
}

// NewCollectStatisticsTacticWithServices supplies the parameter owner when it
// is not bound to the AST manager or process services.
func NewCollectStatisticsTacticWithServices(m *ast.ASTManager, context *uparams.ParamsContext, p *uparams.ParamsRef, output io.Writer) (*CollectStatisticsTactic, error) {
	if m == nil || context == nil || output == nil {
		return nil, uparams.ErrParamsOwner
	}
	if _, _, err := m.RewriterParameterServices(); err != nil {
		return nil, err
	}
	if err := context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	return newCollectStatisticsTactic(m, context, p, output)
}

func collectStatisticsParamsContext(m *ast.ASTManager) (*uparams.ParamsContext, error) {
	context, _, err := m.RewriterParameterServices()
	if err != nil {
		return nil, err
	}
	if context == nil {
		if global := uparams.BorrowProcessGlobalParamsService(); global != nil && global.Context.CheckServices(m.Symbols, m.NumeralService()) == nil {
			context = global.Context
		}
	}
	return context, nil
}

func newCollectStatisticsTactic(m *ast.ASTManager, context *uparams.ParamsContext, p *uparams.ParamsRef, output io.Writer) (*CollectStatisticsTactic, error) {
	if p != nil && context == nil {
		return nil, uparams.ErrParamsOwner
	}
	amount := uint64(reflect.TypeOf(CollectStatisticsTactic{}).Size())
	if err := m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	t := &CollectStatisticsTactic{manager: m, context: context, output: output, stats: make(map[string]uint64), ownerBytes: amount}
	t.self = t
	if context != nil {
		var err error
		t.params, err = context.NewValue()
		if err == nil {
			t.paramsReady = true
			if p != nil {
				err = context.Assign(&t.params, p)
			}
		}
		if err != nil {
			return nil, errors.Join(err, t.Close())
		}
	}
	return t, nil
}

func (t *CollectStatisticsTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *CollectStatisticsTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *CollectStatisticsTactic) DecRef() error {
	if e := t.open(); e != nil {
		return e
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
func (t *CollectStatisticsTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.paramsReady {
		if err := t.context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}

func (t *CollectStatisticsTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("collect-statistics", goal)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	collector := collectStatisticsTraversal{owner: t, seenExpr: make(map[ast.ASTHandle]bool), seenSort: make(map[ast.ASTHandle]bool), seenDecl: make(map[ast.ASTHandle]bool)}
	n, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		h, e := goal.Form(i)
		if e != nil {
			return e
		}
		if e = collector.visit(h); e != nil {
			return e
		}
	}
	if err = t.write(); err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}

func (t *CollectStatisticsTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if t.context == nil {
		context, err := collectStatisticsParamsContext(t.manager)
		if err != nil {
			return err
		}
		if context == nil {
			return uparams.ErrParamsOwner
		}
		t.params, err = context.NewValue()
		if err != nil {
			return err
		}
		t.context, t.paramsReady = context, true
	}
	return t.context.Assign(&t.params, p)
}
func (t *CollectStatisticsTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *CollectStatisticsTactic) CollectStatistics() map[string]float64         { return nil }

// Source reset_statistics calls reset/cleanup, both no-ops; accumulated output
// counters intentionally survive this virtual reset.
func (t *CollectStatisticsTactic) ResetStatistics() error                 { return t.open() }
func (t *CollectStatisticsTactic) Cleanup() error                         { return t.open() }
func (t *CollectStatisticsTactic) Reset() error                           { return t.open() }
func (t *CollectStatisticsTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *CollectStatisticsTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *CollectStatisticsTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, _, err := m.RewriterParameterServices(); err != nil {
		return nil, err
	}
	if t.paramsReady {
		return newCollectStatisticsTactic(m, t.context, &t.params, t.output)
	}
	return newCollectStatisticsTactic(m, nil, nil, t.output)
}
