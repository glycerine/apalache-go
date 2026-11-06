package api

// Source: src/api/api_tactic.cpp:242-312.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type APIProbeComparison uint8

const (
	APIProbeLess APIProbeComparison = iota
	APIProbeGreater
	APIProbeLessEqual
	APIProbeGreaterEqual
	APIProbeEqual
)

func apiProbePair(left, right *APIProbe) (*smtContextState, error) {
	if err := left.open(); err != nil {
		return nil, err
	}
	if err := right.open(); err != nil {
		return nil, err
	}
	if left.owner != right.owner {
		return nil, ErrSMTContextOwner
	}
	return left.owner, nil
}

func (left *APIProbe) Compare(kind APIProbeComparison, right *APIProbe) (*APIProbe, error) {
	owner, err := apiProbePair(left, right)
	if err != nil {
		return nil, err
	}
	var native tactic.ProbeKind
	switch kind {
	case APIProbeLess:
		native = tactic.ProbeLess
	case APIProbeGreater:
		native = tactic.ProbeGreater
	case APIProbeLessEqual:
		native = tactic.ProbeLessEqual
	case APIProbeGreaterEqual:
		native = tactic.ProbeGreaterEqual
	case APIProbeEqual:
		native = tactic.ProbeEqual
	default:
		return nil, ast.ErrASTDomain
	}
	manager, err := owner.command.Manager()
	if err != nil {
		return nil, err
	}
	probe, err := tactic.NewBinaryProbe(manager.Budget(), native, left.probe, right.probe)
	if err != nil {
		return nil, err
	}
	return newAPIProbe(owner, probe)
}

func (left *APIProbe) And(right *APIProbe) (*APIProbe, error) {
	return left.logical(tactic.ProbeAnd, right)
}
func (left *APIProbe) Or(right *APIProbe) (*APIProbe, error) {
	return left.logical(tactic.ProbeOr, right)
}
func (left *APIProbe) logical(kind tactic.ProbeKind, right *APIProbe) (*APIProbe, error) {
	owner, err := apiProbePair(left, right)
	if err != nil {
		return nil, err
	}
	manager, err := owner.command.Manager()
	if err != nil {
		return nil, err
	}
	probe, err := tactic.NewBinaryProbe(manager.Budget(), kind, left.probe, right.probe)
	if err != nil {
		return nil, err
	}
	return newAPIProbe(owner, probe)
}

func (p *APIProbe) Not() (*APIProbe, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	manager, err := p.owner.command.Manager()
	if err != nil {
		return nil, err
	}
	probe, err := tactic.NewNotProbe(manager.Budget(), p.probe)
	if err != nil {
		return nil, err
	}
	return newAPIProbe(p.owner, probe)
}
