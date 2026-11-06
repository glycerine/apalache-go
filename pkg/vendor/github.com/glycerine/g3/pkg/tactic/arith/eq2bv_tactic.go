package arith

// Source: src/tactic/arith/eq2bv_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type Eq2BVTactic struct {
	self       *Eq2BVTactic
	manager    *ast.ASTManager
	trace      *diagnostic.TraceTags
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	bounds     *BoundManager
	ownerBytes uint64
	refs       uint32
	closed     bool
}

var _ tactic.Tactic = (*Eq2BVTactic)(nil)

func NewEq2BVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*Eq2BVTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(Eq2BVTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &Eq2BVTactic{manager: m, context: c, globals: g, ownerBytes: n}
	t.self = t
	var err error
	t.bounds, err = NewBoundManager(m, c, g)
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func MakeEq2BVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (tactic.Tactic, error) {
	child, err := NewEq2BVTactic(m, c, g)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func (t *Eq2BVTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.bounds == nil {
		return arena.ErrClosed
	}
	return nil
}
func (t *Eq2BVTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *Eq2BVTactic) DecRef() error {
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
func (t *Eq2BVTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.bounds != nil {
		if err := t.bounds.Close(); err != nil {
			return err
		}
		t.bounds = nil
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *Eq2BVTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.bounds.Reset()
}
func (t *Eq2BVTactic) Reset() error                                  { return t.Cleanup() }
func (t *Eq2BVTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *Eq2BVTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *Eq2BVTactic) CollectStatistics() map[string]float64         { return nil }
func (t *Eq2BVTactic) ResetStatistics() error                        { return t.open() }
func (t *Eq2BVTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *Eq2BVTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *Eq2BVTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *Eq2BVTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewEq2BVTactic(m, t.context, t.globals)
}
func (t *Eq2BVTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || goal.Manager() != t.manager || result == nil {
		return ast.ErrASTHandle
	}
	if ast.ASTSourceDebug {
		ok, e := goal.IsWellSorted()
		if e != nil {
			return e
		}
		if !ok {
			return ast.ErrASTDomain
		}
	}
	report, err := tactic.NewTacticReport("eq2bv", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	return eq2bvApply(goal, t.bounds, result, t.trace)
}
