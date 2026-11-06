package tactic

// Source: src/tactic/tactic.cpp lazy_tactic; src/tactic/tactic.h mk_lazy_tactic.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type LazyTacticFactory func(*ast.ASTManager, *uparams.ParamsRef) (Tactic, error)

type lazyTactic struct {
	self    *lazyTactic
	manager *ast.ASTManager
	context *uparams.ParamsContext
	params  uparams.ParamsRef
	factory LazyTacticFactory
	child   Tactic
	refs    uint32
	closed  bool
}

func NewLazyTactic(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef, factory LazyTacticFactory) (*lazyTactic, error) {
	if m == nil || c == nil || factory == nil {
		return nil, ast.ErrASTHandle
	}
	snapshot, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	if p != nil {
		if err = c.Copy(&snapshot, p); err != nil {
			_ = c.Del(&snapshot)
			return nil, err
		}
	}
	t := &lazyTactic{manager: m, context: c, params: snapshot, factory: factory}
	t.self = t
	return t, nil
}

func (t *lazyTactic) open() error {
	if t == nil || t.self != t || t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *lazyTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *lazyTactic) DecRef() error {
	if e := t.open(); e != nil {
		return e
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs != 0 {
		return nil
	}
	t.closed = true
	var err error
	if t.child != nil {
		err = t.child.DecRef()
		t.child = nil
	}
	err = errors.Join(err, t.context.Del(&t.params))
	t.factory = nil
	t.manager = nil
	t.context = nil
	return err
}

func (t *lazyTactic) ensure() error {
	if e := t.open(); e != nil {
		return e
	}
	if t.child != nil {
		return nil
	}
	child, err := t.factory(t.manager, &t.params)
	if err != nil {
		return err
	}
	if child == nil {
		return ast.ErrASTHandle
	}
	if err = child.IncRef(); err != nil {
		return err
	}
	if err = child.UpdateParams(&t.params); err != nil {
		return errors.Join(err, child.DecRef())
	}
	t.child = child
	return nil
}

func (t *lazyTactic) Apply(g *Goal, out *TacticResult) error {
	if e := t.ensure(); e != nil {
		return e
	}
	return t.child.Apply(g, out)
}
func (t *lazyTactic) UpdateParams(p *uparams.ParamsRef) error {
	if e := t.open(); e != nil {
		return e
	}
	if p != nil {
		if e := t.context.Append(&t.params, p); e != nil {
			return e
		}
	}
	if t.child != nil {
		return t.child.UpdateParams(p)
	}
	return nil
}
func (t *lazyTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *lazyTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *lazyTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
