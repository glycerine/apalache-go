package arith

// Source: src/tactic/smtlogics/qflia_tactic.cpp quasi_pb_probe.

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type QuasiPBProbe struct {
	self    *QuasiPBProbe
	budget  *arena.Budget
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
	pending []*BoundManager
	refs    uint32
	header  uint64
	closed  bool
}

var _ tactic.ProbeEvaluator = (*QuasiPBProbe)(nil)

func NewQuasiPBProbe(b *arena.Budget, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*QuasiPBProbe, error) {
	if b == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	n := uint64(reflect.TypeOf(QuasiPBProbe{}).Size())
	if err := b.Reserve(n); err != nil {
		return nil, err
	}
	p := &QuasiPBProbe{budget: b, context: c, globals: g, header: n}
	p.self = p
	return p, nil
}
func (p *QuasiPBProbe) Check() error {
	if p == nil || p.self != p || p.closed {
		return tactic.ErrRegistryOwner
	}
	return nil
}
func (p *QuasiPBProbe) IncRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	p.refs++
	return nil
}
func (p *QuasiPBProbe) DecRef() error {
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
func (p *QuasiPBProbe) closePending() error {
	for len(p.pending) > 0 {
		i := len(p.pending) - 1
		if err := p.pending[i].Close(); err != nil {
			return err
		}
		p.pending = p.pending[:i]
	}
	return nil
}
func (p *QuasiPBProbe) Close() error {
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

// QuasiPBProbe checks the bound-manager variables only. The public strategy
// combines this with NOT is-unbounded, preserving the source short circuit.
func (p *QuasiPBProbe) Eval(g *tactic.Goal) (result float64, err error) {
	if err = p.Check(); err != nil {
		return 0, err
	}
	if g == nil || g.Manager() == nil {
		return 0, ast.ErrASTHandle
	}
	bm, err := NewBoundManager(g.Manager(), p.context, p.globals)
	if err != nil {
		return 0, err
	}
	p.pending = append(p.pending, bm)
	defer func() { err = errors.Join(err, p.closePending()) }()
	if err = bm.CollectGoal(g); err != nil {
		return 0, err
	}
	variables, err := bm.Variables()
	if err != nil {
		return 0, err
	}
	foundNon01 := false
	one := big.NewRat(1, 1)
	for _, h := range variables {
		l, _, hasLower, e := bm.Lower(h)
		if e != nil {
			return 0, e
		}
		u, _, hasUpper, e := bm.Upper(h)
		if e != nil {
			return 0, e
		}
		if hasLower && hasUpper && (l.Sign() == 0 || l.Cmp(one) == 0) && (u.Sign() == 0 || u.Cmp(one) == 0) {
			continue
		}
		if foundNon01 {
			return 0, nil
		}
		foundNon01 = true
	}
	return 1, nil
}
