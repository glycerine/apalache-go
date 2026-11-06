package api

// Source: src/api/api_tactic.cpp:109-183,203-219.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func apiTacticPair(first, second *APITactic) (*smtContextState, error) {
	if err := first.open(); err != nil {
		return nil, err
	}
	if err := second.open(); err != nil {
		return nil, err
	}
	if first.owner != second.owner {
		return nil, ErrSMTContextOwner
	}
	return first.owner, nil
}

func (first *APITactic) AndThen(second *APITactic) (*APITactic, error) {
	owner, err := apiTacticPair(first, second)
	if err != nil {
		return nil, err
	}
	value, err := tactic.NewAndThenTactic(first.tactic, second.tactic)
	if err != nil {
		return nil, err
	}
	return newAPITactic(owner, value)
}

func (first *APITactic) OrElse(second *APITactic) (*APITactic, error) {
	owner, err := apiTacticPair(first, second)
	if err != nil {
		return nil, err
	}
	value, err := tactic.NewOrElseTactic(first.tactic, second.tactic)
	if err != nil {
		return nil, err
	}
	return newAPITactic(owner, value)
}

func (t *APITactic) TryFor(milliseconds uint32) (*APITactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	value, err := tactic.NewTryForTactic(t.tactic, milliseconds)
	if err != nil {
		return nil, err
	}
	return newAPITactic(t.owner, value)
}

func (t *APITactic) Repeat(maximum uint32) (*APITactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	value, err := tactic.NewRepeatTactic(t.tactic, maximum)
	if err != nil {
		return nil, err
	}
	return newAPITactic(t.owner, value)
}

func (t *APITactic) When(probe *APIProbe) (*APITactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if err := probe.open(); err != nil {
		return nil, err
	}
	if t.owner != probe.owner {
		return nil, ErrSMTContextOwner
	}
	value, err := tactic.NewWhenTactic(probe.probe, t.tactic)
	if err != nil {
		return nil, err
	}
	return newAPITactic(t.owner, value)
}

func (probe *APIProbe) Conditional(thenTactic, elseTactic *APITactic) (*APITactic, error) {
	if err := probe.open(); err != nil {
		return nil, err
	}
	owner, err := apiTacticPair(thenTactic, elseTactic)
	if err != nil {
		return nil, err
	}
	if probe.owner != owner {
		return nil, ErrSMTContextOwner
	}
	value, err := tactic.NewCondTactic(probe.probe, thenTactic.tactic, elseTactic.tactic)
	if err != nil {
		return nil, err
	}
	return newAPITactic(owner, value)
}

func (probe *APIProbe) FailIf() (*APITactic, error) {
	if err := probe.open(); err != nil {
		return nil, err
	}
	value, err := tactic.NewFailIfTactic(probe.probe)
	if err != nil {
		return nil, err
	}
	return newAPITactic(probe.owner, value)
}

func (t *APITactic) validateOwner(owner *smtContextState) error {
	if err := t.open(); err != nil {
		return err
	}
	if owner == nil || owner != t.owner {
		return ast.ErrASTHandle
	}
	return nil
}
