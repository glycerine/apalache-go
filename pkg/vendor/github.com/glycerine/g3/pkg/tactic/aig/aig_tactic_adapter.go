package aig

// Source: src/tactic/aig/aig_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Tactic retains scalar policy only. Each application obtains its AST manager
// from the goal, just like the source manager-independent aig_tactic factory.
type Tactic struct {
	self       *Tactic
	context    *uparams.ParamsContext
	pass       *Pass
	ownerBytes uint64
	refs       uint32
	closed     bool
}

var _ tactic.Tactic = (*Tactic)(nil)

func NewTactic(c *uparams.ParamsContext, p *uparams.ParamsRef, cooperation *cooperate.Cooperation) (*Tactic, error) {
	if c == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(c.SymbolsField, c.Numerals()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(Tactic{}).Size())
	if err := c.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &Tactic{context: c, pass: NewPass(), ownerBytes: bytes}
	t.self = t
	err := t.UpdateParams(p)
	if err == nil && cooperation != nil {
		err = t.pass.BindCooperation(cooperation)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func MakeTactic(c *uparams.ParamsContext, p *uparams.ParamsRef, cooperation *cooperate.Cooperation) (tactic.Tactic, error) {
	child, err := NewTactic(c, p, cooperation)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func (t *Tactic) open() error {
	if t == nil || t.self != t {
		return ErrOwner
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.CheckServices(t.context.SymbolsField, t.context.Numerals())
}
func (t *Tactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *Tactic) DecRef() error {
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
func (t *Tactic) Close() error {
	if t == nil || t.self != t {
		return ErrOwner
	}
	if t.closed {
		return nil
	}
	if err := t.pass.Close(); err != nil {
		return err
	}
	t.context.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *Tactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || result == nil {
		return ast.ErrASTHandle
	}
	if err = g.CheckOwner(); err != nil {
		return err
	}
	m := g.Manager()
	if err = t.context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return err
	}
	// Source refuses proofs before clearing converters or changing the goal.
	if err = tactic.FailIfProofGeneration("aig", g); err != nil {
		return err
	}
	if err = result.Prepare(m); err != nil {
		return err
	}
	if err = t.pass.apply(g); err != nil {
		return err
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(g)
}
func (t *Tactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		empty := t.context.EmptyValue()
		p = &empty
	}
	return t.pass.UpdateParams(t.context, p)
}
func (t *Tactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.pass.CollectParamDescrs(out)
}
func (t *Tactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if err := t.context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	other, err := NewTactic(t.context, nil, t.pass.cooperation)
	if err != nil {
		return nil, err
	}
	other.pass.DefaultGateEncoding = t.pass.DefaultGateEncoding
	other.pass.PerAssertion = t.pass.PerAssertion
	other.pass.maxMemory = t.pass.maxMemory
	return other, nil
}
func (t *Tactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.pass.Close()
}
func (t *Tactic) Reset() error                           { return t.Cleanup() }
func (t *Tactic) CollectStatistics() map[string]float64  { return nil }
func (t *Tactic) ResetStatistics() error                 { return t.open() }
func (t *Tactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *Tactic) SetProgressCallback(func(uint32)) error { return t.open() }
