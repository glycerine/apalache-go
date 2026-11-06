package core

// Source: src/tactic/core/injectivity_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type InjectivityTactic struct {
	self        *InjectivityTactic
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	params      uparams.ParamsRef
	paramsReady bool
	relations   *injectivityRelations
	config      injectivityRewriteConfiguration
	traversal   *rewriter.Traversal
	ownerBytes  uint64
	refs        uint32
	closed      bool
	running     bool
}

var _ tactic.Tactic = (*InjectivityTactic)(nil)

func NewInjectivityTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (_ *InjectivityTactic, err error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(InjectivityTactic{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &InjectivityTactic{manager: m, context: c, globals: g, ownerBytes: n}
	t.self = t
	t.config.owner = t
	defer func() {
		if err != nil {
			err = errors.Join(err, t.Close())
		}
	}()
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.relations, err = newInjectivityRelations(m)
	}
	if err == nil {
		t.traversal, err = rewriter.NewTraversal(m, &t.config)
	}
	return t, err
}

func MakeInjectivityTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (tactic.Tactic, error) {
	return NewInjectivityTactic(m, c, g, p)
}

func (t *InjectivityTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.relations == nil || t.traversal == nil {
		return arena.ErrClosed
	}
	if t.running {
		return ast.ErrASTPending
	}
	return t.context.Check(&t.params)
}
func (t *InjectivityTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *InjectivityTactic) DecRef() error {
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
func (t *InjectivityTactic) Close() (err error) {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.running {
		return ast.ErrASTPending
	}
	if t.traversal != nil {
		if err = t.traversal.Close(); err != nil {
			return err
		}
		t.traversal = nil
	}
	if t.relations != nil {
		if err = t.relations.Close(); err != nil {
			return err
		}
		t.relations = nil
	}
	if t.paramsReady {
		if err = t.context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *InjectivityTactic) Cleanup() (err error) {
	if err = t.open(); err != nil {
		return err
	}
	fresh, err := newInjectivityRelations(t.manager)
	if err != nil {
		return err
	}
	old := t.relations
	t.relations = fresh
	if err = t.traversal.Reset(); err != nil {
		t.relations = old
		return errors.Join(err, fresh.Close())
	}
	return old.Close()
}
func (t *InjectivityTactic) Reset() error { return t.Cleanup() }
func (t *InjectivityTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewInjectivityTactic(m, t.context, t.globals, &t.params)
}
func (t *InjectivityTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.context.Assign(&t.params, p)
}
func (t *InjectivityTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	memory := "(default: infty) maximum amount of memory in megabytes."
	models := "(default: false) model generation."
	if err := out.InsertName("max_memory", uparams.ParamUInt, &memory, nil, nil); err != nil {
		return err
	}
	return out.InsertName("produce_models", uparams.ParamBool, &models, nil, nil)
}
func (t *InjectivityTactic) CollectStatistics() map[string]float64  { return nil }
func (t *InjectivityTactic) ResetStatistics() error                 { return t.open() }
func (t *InjectivityTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *InjectivityTactic) SetProgressCallback(func(uint32)) error { return t.open() }
