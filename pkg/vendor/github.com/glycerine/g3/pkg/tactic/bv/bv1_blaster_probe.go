package bv

// Source: src/tactic/bv/bv1_blaster_tactic.cpp is_qfbv_eq_probe.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type IsQFBVEqualityProbe struct {
	self    *IsQFBVEqualityProbe
	budget  *arena.Budget
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
	pending []*BV1BlasterTactic
	refs    uint32
	header  uint64
	closed  bool
}

var _ tactic.ProbeEvaluator = (*IsQFBVEqualityProbe)(nil)

func NewIsQFBVEqualityProbe(b *arena.Budget, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*IsQFBVEqualityProbe, error) {
	if b == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	n := uint64(reflect.TypeOf(IsQFBVEqualityProbe{}).Size())
	if err := b.Reserve(n); err != nil {
		return nil, err
	}
	p := &IsQFBVEqualityProbe{budget: b, context: c, globals: g, header: n}
	p.self = p
	return p, nil
}
func (p *IsQFBVEqualityProbe) Check() error {
	if p == nil || p.self != p || p.closed {
		return tactic.ErrRegistryOwner
	}
	return nil
}
func (p *IsQFBVEqualityProbe) IncRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	p.refs++
	return nil
}
func (p *IsQFBVEqualityProbe) DecRef() error {
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
func (p *IsQFBVEqualityProbe) closePending() error {
	for len(p.pending) > 0 {
		i := len(p.pending) - 1
		if err := p.pending[i].Close(); err != nil {
			return err
		}
		p.pending = p.pending[:i]
	}
	return nil
}
func (p *IsQFBVEqualityProbe) Close() error {
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
func (p *IsQFBVEqualityProbe) Eval(g *tactic.Goal) (result float64, err error) {
	if err = p.Check(); err != nil {
		return 0, err
	}
	if g == nil || g.Manager() == nil {
		return 0, ast.ErrASTHandle
	}
	t, err := NewBV1BlasterTactic(g.Manager(), p.context, p.globals, nil, nil)
	if err != nil {
		return 0, err
	}
	p.pending = append(p.pending, t)
	defer func() { err = errors.Join(err, p.closePending()) }()
	yes, err := t.IsTarget(g)
	if yes {
		return 1, err
	}
	return 0, err
}
