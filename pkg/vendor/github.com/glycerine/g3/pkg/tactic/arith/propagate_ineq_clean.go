package arith

// Source: src/tactic/arith/propagate_ineqs_tactic.cpp:60-62;
// src/tactic/tactical.cpp:1107-1128.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
)

// CleanPropagateIneqTactic is the concrete clean(...) factory result. It owns
// the wrapped tactic and rebuilds its implementation after a successful call.
type CleanPropagateIneqTactic struct {
	self   *CleanPropagateIneqTactic
	core   *PropagateIneqTactic
	closed bool
}

// A nil parameter value selects the source factory's default empty params_ref.
func NewCleanPropagateIneqTactic(m *ast.ASTManager, numbers *numeral.RationalManager, context *params.ParamsContext, value *params.ParamsRef) (*CleanPropagateIneqTactic, error) {
	core, err := newPropagateIneqTactic(m, numbers, context, value, 61)
	if err != nil {
		return nil, err
	}
	t := &CleanPropagateIneqTactic{core: core}
	t.self = t
	return t, nil
}

func (t *CleanPropagateIneqTactic) open() error {
	if t == nil || t.self != t || t.core == nil {
		return arena.ErrInvalidSlot
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.core.open()
}

func (t *CleanPropagateIneqTactic) Apply(input *goaltactic.Goal) (*goaltactic.Goal, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	result, err := t.core.Apply(input)
	if err != nil {
		return result, err
	}
	return result, t.core.Cleanup()
}

func (t *CleanPropagateIneqTactic) UpdateParams(value *params.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.core.UpdateParams(value)
}

func (t *CleanPropagateIneqTactic) Translate(m *ast.ASTManager, numbers *numeral.RationalManager) (*CleanPropagateIneqTactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	core, err := t.core.Translate(m, numbers)
	if err != nil {
		return nil, err
	}
	out := &CleanPropagateIneqTactic{core: core}
	out.self = out
	return out, nil
}

func (t *CleanPropagateIneqTactic) Close() error {
	if t == nil || t.self != t || t.core == nil {
		return arena.ErrInvalidSlot
	}
	if t.closed {
		return nil
	}
	if err := t.core.Close(); err != nil {
		return err
	}
	t.closed = true
	return nil
}
