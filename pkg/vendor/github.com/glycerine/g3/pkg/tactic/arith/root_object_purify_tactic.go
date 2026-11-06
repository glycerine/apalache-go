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

type RootObjectPurifyTactic struct {
	self       *RootObjectPurifyTactic
	manager    *ast.ASTManager
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	services   model.ModelEvaluatorConfig
	params     uparams.ParamsRef
	arithmetic ast.FamilyID
	elim       bool
	refs       uint32
	ownerBytes uint64
	closed     bool
}

var _ tactic.Tactic = (*RootObjectPurifyTactic)(nil)

func NewRootObjectPurifyTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (_ *RootObjectPurifyTactic, err error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g {
		return nil, uparams.ErrParamsOwner
	}
	v, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(RootObjectPurifyTactic{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, errors.Join(err, c.Del(&v))
	}
	t := &RootObjectPurifyTactic{manager: m, context: c, globals: g, services: services, params: v, ownerBytes: n}
	t.self = t
	if p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.arithmetic, err = m.RegisterArithmeticPlugin()
	}
	if err == nil {
		t.elim, err = t.readPolicy(&t.params)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func (t *RootObjectPurifyTactic) readPolicy(p *uparams.ParamsRef) (bool, error) {
	name := "elim_root_objects"
	return t.context.GetBoolName(p, &name, true)
}
func (t *RootObjectPurifyTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *RootObjectPurifyTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *RootObjectPurifyTactic) DecRef() error {
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
func (t *RootObjectPurifyTactic) UpdateParams(p *uparams.ParamsRef) error {
	if e := t.open(); e != nil {
		return e
	}
	if e := t.context.Assign(&t.params, p); e != nil {
		return e
	}
	var e error
	t.elim, e = t.readPolicy(p)
	return e
}
func (t *RootObjectPurifyTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	d := "(default: true) eliminate root objects"
	return out.InsertName("elim_root_objects", uparams.ParamBool, &d, nil, nil)
}
func (t *RootObjectPurifyTactic) CollectStatistics() map[string]float64  { return nil }
func (t *RootObjectPurifyTactic) ResetStatistics() error                 { return t.open() }
func (t *RootObjectPurifyTactic) Cleanup() error                         { return t.open() }
func (t *RootObjectPurifyTactic) Reset() error                           { return t.open() }
func (t *RootObjectPurifyTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *RootObjectPurifyTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *RootObjectPurifyTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	return NewRootObjectPurifyTactic(m, t.context, t.globals, &t.params, t.services)
}
func (t *RootObjectPurifyTactic) Close() error {
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
