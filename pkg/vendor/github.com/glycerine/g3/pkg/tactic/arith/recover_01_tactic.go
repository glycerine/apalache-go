package arith

// Source: src/tactic/arith/recover_01_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type Recover01Tactic struct {
	self                *Recover01Tactic
	manager             *ast.ASTManager
	context             *uparams.ParamsContext
	globals             *uparams.GlobalParamsService
	services            model.ModelEvaluatorConfig
	trace               *diagnostic.TraceTags
	params              uparams.ParamsRef
	rewriter            *rewriter.TheoryRewriter
	maxBits             uint32
	ownerBytes          uint64
	refs                uint32
	paramsReady, closed bool
}

var _ tactic.Tactic = (*Recover01Tactic)(nil)

func NewRecover01Tactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*Recover01Tactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(Recover01Tactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &Recover01Tactic{manager: m, context: c, globals: g, services: services, ownerBytes: n}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.rewriter, err = rewriter.NewTheoryRewriter(m, c, g, &t.params)
	}
	if err == nil {
		key := "recover_01_max_bits"
		t.maxBits, err = c.GetUIntName(&t.params, &key, 10)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func MakeRecover01Tactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewRecover01Tactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func (t *Recover01Tactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.rewriter == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *Recover01Tactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *Recover01Tactic) DecRef() error {
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
func (t *Recover01Tactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.rewriter != nil {
		if err := t.rewriter.Close(); err != nil {
			return err
		}
		t.rewriter = nil
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
func (t *Recover01Tactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	fresh, err := rewriter.NewTheoryRewriter(t.manager, t.context, t.globals, &t.params)
	if err != nil {
		if fresh != nil {
			err = errors.Join(err, fresh.Close())
		}
		return err
	}
	if err = t.rewriter.Close(); err != nil {
		return errors.Join(err, fresh.Close())
	}
	t.rewriter = fresh
	return nil
}
func (t *Recover01Tactic) Reset() error { return t.Cleanup() }
func (t *Recover01Tactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewRecover01Tactic(m, t.context, t.globals, &t.params, t.services)
}
func (t *Recover01Tactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	if err := t.rewriter.UpdateParams(p); err != nil {
		return err
	}
	key := "recover_01_max_bits"
	value, err := t.context.GetUIntName(p, &key, 10)
	if err == nil {
		t.maxBits = value
	}
	return err
}
func (t *Recover01Tactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := rewriter.TheoryRewriterParameterDescriptions(out); err != nil {
		return err
	}
	description := "(default: 10) maximum number of bits to consider in a clause."
	return out.InsertName("recover_01_max_bits", uparams.ParamUInt, &description, nil, nil)
}
func (t *Recover01Tactic) CollectStatistics() map[string]float64  { return nil }
func (t *Recover01Tactic) ResetStatistics() error                 { return t.open() }
func (t *Recover01Tactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *Recover01Tactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *Recover01Tactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *Recover01Tactic) Apply(goal *tactic.Goal, out *tactic.TacticResult) (err error) {
	defer func() {
		if rewriter.IsRewriterException(err) {
			err = tactic.NewTacticException(err.Error())
		}
	}()
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || goal.Manager() != t.manager || out == nil {
		return ast.ErrASTHandle
	}
	return recover01Apply(goal, t.rewriter, t.services, t.maxBits, out, t.trace)
}
