package fpa

// Source: src/tactic/fpa/fpa2bv_tactic.cpp/.h tactic virtual interface.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// FPA2BVTactic adapts the retained conversion pass to the common tactic
// interface used by tacticals and check-sat-using.
type FPA2BVTactic struct {
	self    *FPA2BVTactic
	manager *ast.ASTManager
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
	params  uparams.ParamsRef
	pass    *Pass
	refs    uint32
	closed  bool
}

var _ tactic.Tactic = (*FPA2BVTactic)(nil)

func NewFPA2BVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*FPA2BVTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	stored, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	t := &FPA2BVTactic{manager: m, context: c, globals: g, params: stored}
	t.self = t
	if p != nil {
		if err = c.Assign(&t.params, p); err != nil {
			return nil, errors.Join(err, t.Close())
		}
	}
	t.pass, err = NewPass(m, c, g, &t.params)
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func (t *FPA2BVTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.pass == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}

func (t *FPA2BVTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}

func (t *FPA2BVTactic) DecRef() error {
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

func (t *FPA2BVTactic) Apply(goal *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || out == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = out.Prepare(t.manager); err != nil {
		return err
	}
	converted, err := t.pass.Apply(goal)
	if err != nil {
		return err
	}
	goalMoved := false
	converterMoved := false
	defer func() {
		if !goalMoved && converted.Goal != nil {
			err = errors.Join(err, converted.Goal.DecRef())
		}
		if !converterMoved && converted.ModelConverter != nil {
			err = errors.Join(err, converted.ModelConverter.Close())
		}
	}()
	if err = out.TakeGoal(converted.Goal); err != nil {
		return err
	}
	goalMoved = true
	if converted.ModelConverter != nil {
		if err = out.SetModelConverter(converted.ModelConverter); err != nil {
			return err
		}
		converterMoved = true
	}
	return nil
}

func (t *FPA2BVTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		return uparams.ErrParamsOwner
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.pass.UpdateParams(p)
}

func (t *FPA2BVTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *FPA2BVTactic) CollectStatistics() map[string]float64         { return nil }
func (t *FPA2BVTactic) ResetStatistics() error                        { return t.open() }
func (t *FPA2BVTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.pass.Cleanup()
}
func (t *FPA2BVTactic) Reset() error                           { return t.Cleanup() }
func (t *FPA2BVTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *FPA2BVTactic) SetProgressCallback(func(uint32)) error { return t.open() }

func (t *FPA2BVTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewFPA2BVTactic(m, t.context, t.globals, &t.params)
}

func (t *FPA2BVTactic) Close() (err error) {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.refs != 0 {
		return arena.ErrBounds
	}
	if t.pass != nil {
		if err = t.pass.Close(); err != nil {
			return err
		}
		t.pass = nil
	}
	if err = t.context.Del(&t.params); err != nil {
		return err
	}
	t.closed = true
	return nil
}
