package arith

// Source: src/tactic/arith/probe_arith.cpp:25-144.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type arithmeticStatistic uint8

const (
	statAverageDegree arithmeticStatistic = iota
	statMaximumDegree
	statAverageBitWidth
	statMaximumBitWidth
)

type ArithmeticStatisticProbe struct {
	self   *ArithmeticStatisticProbe
	budget *arena.Budget
	header uint64
	refs   uint32
	kind   arithmeticStatistic
	closed bool
}

var _ tactic.ProbeEvaluator = (*ArithmeticStatisticProbe)(nil)

func newArithmeticStatisticProbe(b *arena.Budget, k arithmeticStatistic) (*ArithmeticStatisticProbe, error) {
	if b == nil || k > statMaximumBitWidth {
		return nil, ast.ErrASTDomain
	}
	n := uint64(reflect.TypeOf(ArithmeticStatisticProbe{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	p := &ArithmeticStatisticProbe{budget: b, header: n, kind: k}
	p.self = p
	return p, nil
}
func NewArithmeticAverageDegreeProbe(b *arena.Budget) (*ArithmeticStatisticProbe, error) {
	return newArithmeticStatisticProbe(b, statAverageDegree)
}
func NewArithmeticMaximumDegreeProbe(b *arena.Budget) (*ArithmeticStatisticProbe, error) {
	return newArithmeticStatisticProbe(b, statMaximumDegree)
}
func NewArithmeticAverageBitWidthProbe(b *arena.Budget) (*ArithmeticStatisticProbe, error) {
	return newArithmeticStatisticProbe(b, statAverageBitWidth)
}
func NewArithmeticMaximumBitWidthProbe(b *arena.Budget) (*ArithmeticStatisticProbe, error) {
	return newArithmeticStatisticProbe(b, statMaximumBitWidth)
}
func (p *ArithmeticStatisticProbe) Check() error {
	if p == nil || p.self != p || p.closed {
		return tactic.ErrRegistryOwner
	}
	return nil
}
func (p *ArithmeticStatisticProbe) IncRef() error {
	if e := p.Check(); e != nil {
		return e
	}
	p.refs++
	return nil
}
func (p *ArithmeticStatisticProbe) DecRef() error {
	if e := p.Check(); e != nil {
		return e
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
func (p *ArithmeticStatisticProbe) Close() error {
	if p == nil || p.self != p {
		return tactic.ErrRegistryOwner
	}
	if p.closed {
		return nil
	}
	p.budget.Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}
func (p *ArithmeticStatisticProbe) Eval(g *tactic.Goal) (float64, error) {
	if e := p.Check(); e != nil {
		return 0, e
	}
	if g == nil {
		return 0, ast.ErrASTHandle
	}
	if p.kind <= statMaximumDegree {
		return p.evalDegree(g)
	}
	return p.evalBitWidth(g)
}
func statisticResult(avg bool, max uint32, total uint64, count uint32) float64 {
	if avg {
		if count == 0 {
			return 0
		}
		return float64(total) / float64(count)
	}
	return float64(max)
}
