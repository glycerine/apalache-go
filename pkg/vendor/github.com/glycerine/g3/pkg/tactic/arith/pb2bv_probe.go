package arith

// Source: src/tactic/arith/pb2bv_tactic.cpp is_pb_probe/mk_is_pb_probe.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type IsPBProbe struct {
	self       *IsPBProbe
	budget     *arena.Budget
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	ownerBytes uint64
	references uint32
	closed     bool
}

var _ tactic.ProbeEvaluator = (*IsPBProbe)(nil)

func NewIsPBProbe(b *arena.Budget, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*IsPBProbe, error) {
	if b == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	n := uint64(reflect.TypeOf(IsPBProbe{}).Size())
	if err := b.Reserve(n); err != nil {
		return nil, err
	}
	p := &IsPBProbe{budget: b, context: c, globals: g, ownerBytes: n}
	p.self = p
	return p, nil
}
func (p *IsPBProbe) Check() error {
	if p == nil || p.self != p || p.closed {
		return tactic.ErrRegistryOwner
	}
	return nil
}
func (p *IsPBProbe) IncRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	p.references++
	return nil
}
func (p *IsPBProbe) DecRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	if p.references == 0 {
		return tactic.ErrRegistryOwner
	}
	p.references--
	if p.references == 0 {
		return p.Close()
	}
	return nil
}
func (p *IsPBProbe) Close() error {
	if p == nil || p.self != p {
		return tactic.ErrRegistryOwner
	}
	if p.closed {
		return nil
	}
	p.budget.Release(p.ownerBytes)
	p.ownerBytes = 0
	p.closed = true
	return nil
}
func (p *IsPBProbe) Eval(goal *tactic.Goal) (float64, error) {
	if err := p.Check(); err != nil {
		return 0, err
	}
	yes, err := IsPseudoBooleanGoal(goal, p.context, p.globals)
	if yes {
		return 1, err
	}
	return 0, err
}
