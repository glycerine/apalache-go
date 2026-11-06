package api

// Source: src/api/api_tactic.cpp:79-107,233-240.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type APIProbe struct {
	self   *APIProbe
	owner  *smtContextState
	probe  tactic.ProbeEvaluator
	closed bool
}

func newAPIProbe(owner *smtContextState, probe tactic.ProbeEvaluator) (*APIProbe, error) {
	if owner == nil || probe == nil {
		return nil, ast.ErrASTHandle
	}
	if err := probe.IncRef(); err != nil {
		return nil, err
	}
	result := &APIProbe{owner: owner, probe: probe}
	result.self = result
	return result, nil
}

func (p *APIProbe) open() error {
	if p == nil || p.self != p || p.owner == nil || p.probe == nil || p.closed || p.owner.closed || p.owner.command == nil {
		return ErrSMTContextOwner
	}
	return p.probe.Check()
}

func (p *APIProbe) Close() error {
	if p == nil || p.closed {
		return nil
	}
	if p.self != p || p.probe == nil {
		return ErrSMTContextOwner
	}
	err := p.probe.DecRef()
	if err == nil {
		p.closed = true
		p.owner, p.probe = nil, nil
	}
	return err
}

func (p *APIProbe) Retain() (*APIProbe, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	return newAPIProbe(p.owner, p.probe)
}

func (c *SMTContext) ConstantProbe(value float64) (*APIProbe, error) {
	m, err := c.Manager()
	if err != nil {
		return nil, err
	}
	probe, err := tactic.NewProbe(m.Budget(), tactic.ProbeConstant, value)
	if err != nil {
		return nil, err
	}
	result, err := newAPIProbe(c.smtContextState, probe)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type APIGoalProbeKind uint8

const (
	APIGoalProbeDepth APIGoalProbeKind = iota
	APIGoalProbeSize
	APIGoalProbeNumExpressions
	APIGoalProbeProduceProofs
	APIGoalProbeProduceModels
	APIGoalProbeProduceUnsatCores
)

func (c *SMTContext) GoalProbe(kind APIGoalProbeKind) (*APIProbe, error) {
	m, err := c.Manager()
	if err != nil {
		return nil, err
	}
	var native tactic.ProbeKind
	switch kind {
	case APIGoalProbeDepth:
		native = tactic.ProbeDepth
	case APIGoalProbeSize:
		native = tactic.ProbeSize
	case APIGoalProbeNumExpressions:
		native = tactic.ProbeNumExprs
	case APIGoalProbeProduceProofs:
		native = tactic.ProbeProduceProofs
	case APIGoalProbeProduceModels:
		native = tactic.ProbeProduceModels
	case APIGoalProbeProduceUnsatCores:
		native = tactic.ProbeProduceUnsatCores
	default:
		return nil, ast.ErrASTDomain
	}
	probe, err := tactic.NewProbe(m.Budget(), native, 0)
	if err != nil {
		return nil, err
	}
	return newAPIProbe(c.smtContextState, probe)
}
