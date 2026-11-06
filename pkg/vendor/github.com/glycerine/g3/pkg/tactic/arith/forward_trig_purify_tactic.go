package arith

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"reflect"
)

type ForwardTrigPurifyTactic struct {
	self       *ForwardTrigPurifyTactic
	manager    *ast.ASTManager
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	services   model.ModelEvaluatorConfig
	params     uparams.ParamsRef
	arithmetic ast.FamilyID
	refs       uint32
	ownerBytes uint64
	closed     bool
}

var _ tactic.Tactic = (*ForwardTrigPurifyTactic)(nil)

func NewForwardTrigPurifyTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (_ *ForwardTrigPurifyTactic, err error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	v, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(ForwardTrigPurifyTactic{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, errors.Join(err, c.Del(&v))
	}
	t := &ForwardTrigPurifyTactic{manager: m, context: c, globals: g, services: services, params: v, ownerBytes: n}
	t.self = t
	if p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.arithmetic, err = m.RegisterArithmeticPlugin()
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func (t *ForwardTrigPurifyTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *ForwardTrigPurifyTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *ForwardTrigPurifyTactic) DecRef() error {
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
func (t *ForwardTrigPurifyTactic) UpdateParams(p *uparams.ParamsRef) error {
	if e := t.open(); e != nil {
		return e
	}
	return t.context.Assign(&t.params, p)
}
func (t *ForwardTrigPurifyTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *ForwardTrigPurifyTactic) CollectStatistics() map[string]float64         { return nil }
func (t *ForwardTrigPurifyTactic) ResetStatistics() error                        { return t.open() }
func (t *ForwardTrigPurifyTactic) Cleanup() error                                { return t.open() }
func (t *ForwardTrigPurifyTactic) Reset() error                                  { return t.open() }
func (t *ForwardTrigPurifyTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *ForwardTrigPurifyTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *ForwardTrigPurifyTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	return NewForwardTrigPurifyTactic(m, t.context, t.globals, &t.params, t.services)
}
func (t *ForwardTrigPurifyTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	e := t.context.Del(&t.params)
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return e
}
