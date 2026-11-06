package bv

// Source: src/tactic/bv/bv_slice_tactic.h; src/ast/simplifiers/bv_slice.h.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type BVSliceTactic struct {
	self            *BVSliceTactic
	manager         *ast.ASTManager
	context         *uparams.ParamsContext
	globals         *uparams.GlobalParamsService
	services        model.ModelEvaluatorConfig
	params          uparams.ParamsRef
	paramsReady     bool
	theory          *rewriter.TheoryRewriter
	refs            uint32
	closed, running bool
}

var _ tactic.Tactic = (*BVSliceTactic)(nil)

func NewBVSliceTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (t *BVSliceTactic, err error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	t = &BVSliceTactic{manager: m, context: c, globals: g, services: services}
	t.self = t
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
		t.theory, err = rewriter.NewTheoryRewriter(m, c, g, &t.params)
	}
	return t, err
}
func (t *BVSliceTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.running {
		return ast.ErrASTPending
	}
	return t.context.Check(&t.params)
}
func (t *BVSliceTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *BVSliceTactic) DecRef() error {
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
func (t *BVSliceTactic) Close() (err error) {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.running {
		return ast.ErrASTPending
	}
	if t.theory != nil {
		err = errors.Join(err, t.theory.Close())
		t.theory = nil
	}
	if t.paramsReady {
		err = errors.Join(err, t.context.Del(&t.params))
		t.paramsReady = false
	}
	t.closed = true
	return err
}
func (t *BVSliceTactic) Cleanup() error {
	if e := t.open(); e != nil {
		return e
	}
	return t.theory.Cleanup()
}
func (t *BVSliceTactic) Reset() error { return t.Cleanup() }
func (t *BVSliceTactic) UpdateParams(p *uparams.ParamsRef) error {
	if e := t.open(); e != nil {
		return e
	}
	if p == nil {
		return uparams.ErrParamsOwner
	}
	if e := t.context.Assign(&t.params, p); e != nil {
		return e
	}
	return t.theory.UpdateParams(&t.params)
}
func (t *BVSliceTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if e := t.open(); e != nil {
		return e
	}
	return rewriter.TheoryRewriterParameterDescriptions(out)
}
func (t *BVSliceTactic) CollectStatistics() map[string]float64  { return nil }
func (t *BVSliceTactic) ResetStatistics() error                 { return t.open() }
func (t *BVSliceTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *BVSliceTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *BVSliceTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	return NewBVSliceTactic(m, t.context, t.globals, &t.params, t.services)
}
