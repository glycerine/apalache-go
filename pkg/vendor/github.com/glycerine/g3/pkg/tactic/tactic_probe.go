package tactic

// Source: src/tactic/probe.h:30-61; src/tactic/probe.cpp:39-70,180-192.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

var ErrRegistryOwner = errors.New("tactic: invalid, copied or retired registration owner")

type ProbeKind uint8

// ProbeEvaluator is the retained source probe virtual boundary. Concrete
// arithmetic probes can participate in the same typed compositions without
// introducing a dependency from base tactics back to arithmetic tactics.
type ProbeEvaluator interface {
	Check() error
	IncRef() error
	DecRef() error
	Close() error
	Eval(*Goal) (float64, error)
}

const (
	ProbeConstant ProbeKind = iota
	ProbeDepth
	ProbeSize
	ProbeNumExprs
	ProbeMemory
	ProbeNot
	ProbeAnd
	ProbeOr
	ProbeImplies
	ProbeEqual
	ProbeNotEqual
	ProbeLessEqual
	ProbeLess
	ProbeGreaterEqual
	ProbeGreater
	ProbeAdd
	ProbeSubtract
	ProbeMultiply
	ProbeDivide
	ProbeProduceProofs
	ProbeProduceModels
	ProbeProduceUnsatCores
	ProbeNumConsts
	ProbeNumBoolConsts
	ProbeNumArithmeticConsts
	ProbeNumBitVectorConsts
	ProbeHasQuantifier
	ProbeHasPattern
	ProbeIsPropositional
	ProbeIsQFBitVector
	ProbeIsQFArrayUFBitVector
	ProbeIsQFUFBitVector
)

// Concrete source probe classes share cold reference metadata. Each Eval
// branch is the original class body, borrowing the goal only for that call.
type Probe struct {
	self       *Probe
	budget     *arena.Budget
	constant   float64
	left       ProbeEvaluator
	right      ProbeEvaluator
	ownerBytes uint64
	References uint32
	kind       ProbeKind
	Closed     bool
}

func NewProbe(b *arena.Budget, kind ProbeKind, value float64) (*Probe, error) {
	if b == nil || kind > ProbeIsQFUFBitVector || kind > ProbeMemory && kind < ProbeProduceProofs {
		return nil, ast.ErrASTDomain
	}
	n := uint64(reflect.TypeOf(Probe{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	p := &Probe{budget: b, kind: kind, constant: value, ownerBytes: n}
	p.self = p
	return p, nil
}

func newComposedProbe(b *arena.Budget, kind ProbeKind, left, right ProbeEvaluator) (p *Probe, err error) {
	if b == nil || left == nil {
		return nil, ast.ErrASTHandle
	}
	if err = left.Check(); err != nil {
		return nil, err
	}
	if kind != ProbeNot && right == nil {
		return nil, ast.ErrASTHandle
	}
	if right != nil {
		if err = right.Check(); err != nil {
			return nil, err
		}
	}
	n := uint64(reflect.TypeOf(Probe{}).Size())
	if err = b.Reserve(n); err != nil {
		return nil, err
	}
	p = &Probe{budget: b, kind: kind, left: left, right: right, ownerBytes: n}
	p.self = p
	if err = left.IncRef(); err != nil {
		b.Release(n)
		return nil, err
	}
	if right != nil {
		if err = right.IncRef(); err != nil {
			_ = left.DecRef()
			b.Release(n)
			return nil, err
		}
	}
	return p, nil
}

func NewNotProbe(b *arena.Budget, p ProbeEvaluator) (*Probe, error) {
	return newComposedProbe(b, ProbeNot, p, nil)
}

func NewBinaryProbe(b *arena.Budget, kind ProbeKind, left, right ProbeEvaluator) (*Probe, error) {
	if kind < ProbeAnd || kind > ProbeDivide {
		return nil, ast.ErrASTDomain
	}
	return newComposedProbe(b, kind, left, right)
}
func (p *Probe) open() error {
	if p == nil || p.self != p || p.Closed {
		return ErrRegistryOwner
	}
	return nil
}

func (p *Probe) Check() error { return p.open() }
func (p *Probe) IncRef() error {
	if e := p.open(); e != nil {
		return e
	}
	p.References++
	return nil
}
func (p *Probe) DecRef() error {
	if e := p.open(); e != nil {
		return e
	}
	if p.References == 0 {
		return ErrRegistryOwner
	}
	p.References--
	if p.References == 0 {
		return p.Close()
	}
	return nil
}
func (p *Probe) Close() error {
	if p == nil || p.self != p {
		return ErrRegistryOwner
	}
	if p.Closed {
		return nil
	}
	var err error
	if p.left != nil {
		err = errors.Join(err, p.left.DecRef())
		p.left = nil
	}
	if p.right != nil {
		err = errors.Join(err, p.right.DecRef())
		p.right = nil
	}
	p.budget.Release(p.ownerBytes)
	p.ownerBytes = 0
	p.Closed = true
	return err
}
func (p *Probe) Eval(g *Goal) (float64, error) {
	if e := p.open(); e != nil {
		return 0, e
	}
	if e := g.open(); e != nil {
		return 0, e
	}
	switch p.kind {
	case ProbeConstant:
		return p.constant, nil
	case ProbeDepth:
		return float64(g.depth), nil
	case ProbeSize:
		n, e := g.Size()
		return float64(n), e
	case ProbeNumExprs:
		n, e := g.NumExprs()
		return float64(n), e
	case ProbeMemory:
		numerals := g.manager.NumeralService()
		if numerals == nil || numerals.RuntimeOwner() == nil {
			return 0, ast.ErrASTHandle
		}
		bytes, e := numerals.RuntimeOwner().AllocationSize()
		if e != nil {
			return 0, e
		}
		return float64(bytes) / float64(1024*1024), nil
	case ProbeProduceProofs:
		enabled, e := g.ProofsEnabled()
		return boolProbeValue(enabled), e
	case ProbeProduceModels:
		enabled, e := g.ModelsEnabled()
		return boolProbeValue(enabled), e
	case ProbeProduceUnsatCores:
		enabled, e := g.UnsatCoresEnabled()
		return boolProbeValue(enabled), e
	case ProbeNumConsts, ProbeNumBoolConsts, ProbeNumArithmeticConsts, ProbeNumBitVectorConsts:
		count, e := p.countConstants(g)
		return float64(count), e
	case ProbeHasQuantifier:
		yes, e := p.goalHasQuantifier(g)
		return boolProbeValue(yes), e
	case ProbeHasPattern:
		yes, e := p.goalHasPattern(g)
		return boolProbeValue(yes), e
	case ProbeIsPropositional, ProbeIsQFBitVector, ProbeIsQFArrayUFBitVector, ProbeIsQFUFBitVector:
		yes, e := p.goalInFragment(g)
		return boolProbeValue(yes), e
	case ProbeNot:
		left, e := p.left.Eval(g)
		return boolProbeValue(!ProbeIsTrue(left)), e
	case ProbeAnd:
		left, e := p.left.Eval(g)
		if e != nil || !ProbeIsTrue(left) {
			return boolProbeValue(false), e
		}
		right, e := p.right.Eval(g)
		return boolProbeValue(ProbeIsTrue(right)), e
	case ProbeOr:
		left, e := p.left.Eval(g)
		if e != nil || ProbeIsTrue(left) {
			return boolProbeValue(ProbeIsTrue(left)), e
		}
		right, e := p.right.Eval(g)
		return boolProbeValue(ProbeIsTrue(right)), e
	case ProbeImplies:
		left, e := p.left.Eval(g)
		if e != nil || !ProbeIsTrue(left) {
			return boolProbeValue(true), e
		}
		right, e := p.right.Eval(g)
		return boolProbeValue(ProbeIsTrue(right)), e
	}
	left, e := p.left.Eval(g)
	if e != nil {
		return 0, e
	}
	right, e := p.right.Eval(g)
	if e != nil {
		return 0, e
	}
	switch p.kind {
	case ProbeEqual:
		return boolProbeValue(left == right), nil
	case ProbeNotEqual:
		return boolProbeValue(left != right), nil
	case ProbeLessEqual:
		return boolProbeValue(left <= right), nil
	case ProbeLess:
		return boolProbeValue(left < right), nil
	case ProbeGreaterEqual:
		return boolProbeValue(left >= right), nil
	case ProbeGreater:
		return boolProbeValue(left > right), nil
	case ProbeAdd:
		return left + right, nil
	case ProbeSubtract:
		return left - right, nil
	case ProbeMultiply:
		return left * right, nil
	case ProbeDivide:
		return left / right, nil
	}
	return 0, ast.ErrASTDomain
}
func ProbeIsTrue(value float64) bool { return value != 0 }

func boolProbeValue(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
