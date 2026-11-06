package arith

// Source: src/tactic/arith/add_bounds_tactic.cpp is_unbounded and its probe.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type IsUnboundedProbe struct {
	self    *IsUnboundedProbe
	budget  *arena.Budget
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
	pending []*BoundManager
	refs    uint32
	header  uint64
	closed  bool
}

var _ tactic.ProbeEvaluator = (*IsUnboundedProbe)(nil)

func NewIsUnboundedProbe(b *arena.Budget, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*IsUnboundedProbe, error) {
	if b == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	n := uint64(reflect.TypeOf(IsUnboundedProbe{}).Size())
	if err := b.Reserve(n); err != nil {
		return nil, err
	}
	p := &IsUnboundedProbe{budget: b, context: c, globals: g, header: n}
	p.self = p
	return p, nil
}
func (p *IsUnboundedProbe) Check() error {
	if p == nil || p.self != p || p.closed {
		return tactic.ErrRegistryOwner
	}
	return nil
}
func (p *IsUnboundedProbe) IncRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	p.refs++
	return nil
}
func (p *IsUnboundedProbe) DecRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	if p.refs == 0 {
		return tactic.ErrRegistryOwner
	}
	p.refs--
	if p.refs == 0 {
		return p.Close()
	}
	return nil
}
func (p *IsUnboundedProbe) closePending() error {
	for len(p.pending) > 0 {
		i := len(p.pending) - 1
		if err := p.pending[i].Close(); err != nil {
			return err
		}
		p.pending = p.pending[:i]
	}
	return nil
}
func (p *IsUnboundedProbe) Close() error {
	if p == nil || p.self != p {
		return tactic.ErrRegistryOwner
	}
	if p.closed {
		return nil
	}
	if err := p.closePending(); err != nil {
		return err
	}
	p.budget.Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}
func (p *IsUnboundedProbe) Eval(g *tactic.Goal) (result float64, err error) {
	if err = p.Check(); err != nil {
		return 0, err
	}
	if g == nil {
		return 0, ast.ErrASTHandle
	}
	m := g.Manager()
	if m == nil {
		return 0, ast.ErrASTHandle
	}
	bm, err := NewBoundManager(m, p.context, p.globals)
	if err != nil {
		return 0, err
	}
	p.pending = append(p.pending, bm)
	defer func() { err = errors.Join(err, p.closePending()) }()
	if err = bm.CollectGoal(g); err != nil {
		return 0, err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return 0, err
	}
	found, err := visitArithmeticGoal(g, func(h ast.ASTHandle, kind ast.ASTKind) (bool, error) {
		if kind != ast.ASTApp {
			return false, nil
		}
		_, yes, e := arithmeticConstant(m, h, family)
		if e != nil || !yes {
			return false, e
		}
		lower, e := bm.HasLower(h)
		if e != nil {
			return false, e
		}
		if !lower {
			return true, nil
		}
		upper, e := bm.HasUpper(h)
		return !upper, e
	})
	if found {
		return 1, err
	}
	return 0, err
}

func IsUnboundedGoal(g *tactic.Goal, c *uparams.ParamsContext, globals *uparams.GlobalParamsService) (result bool, err error) {
	if g == nil || g.Manager() == nil {
		return false, ast.ErrASTHandle
	}
	p, err := NewIsUnboundedProbe(g.Manager().Budget(), c, globals)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, p.Close()) }()
	n, err := p.Eval(g)
	return n != 0, err
}
