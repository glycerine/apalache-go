package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:92-120, concrete MPQ branch.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/subpaving"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrRationalEngineSelection = errors.New("subpaving tactic: selected numeral engine is not rational")

// RationalEngine owns the selected MPQ context. Its manager and limit are borrowed.
type RationalEngine struct {
	self    *RationalEngine
	context *subpaving.RationalContext
	shell   *subpaving.RationalShellCoefficients
	numbers *numeral.RationalManager
	options Options
	closed  bool
}

func NewRationalEngine(limit *control.Reslimit, numbers *numeral.RationalManager, pc *params.ParamsContext, p *params.ParamsRef) (*RationalEngine, error) {
	if limit == nil || numbers == nil || pc == nil || p == nil {
		return nil, arena.ErrInvalidSlot
	}
	selected, err := ReadOptions(pc, p)
	if err != nil {
		return nil, err
	}
	if selected.Engine != EngineMPQ {
		return nil, ErrRationalEngineSelection
	}
	context, cause := subpaving.NewRationalContextFromParams(numbers, limit, pc, p)
	if context == nil {
		return nil, cause
	}
	shell, err := subpaving.NewRationalShellCoefficients(context)
	if err != nil {
		return nil, errors.Join(cause, err, context.Close())
	}
	e := &RationalEngine{context: context, shell: shell, numbers: numbers, options: selected}
	e.self = e
	return e, cause
}

func (e *RationalEngine) check() error {
	if e == nil || e.self != e {
		return arena.ErrInvalidSlot
	}
	if e.closed {
		return arena.ErrClosed
	}
	if e.context == nil || e.shell == nil || e.numbers == nil {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (e *RationalEngine) Context() (*subpaving.RationalContext, error) {
	if err := e.check(); err != nil {
		return nil, err
	}
	return e.context, nil
}

func (e *RationalEngine) Shell() (*subpaving.RationalShellCoefficients, error) {
	if err := e.check(); err != nil {
		return nil, err
	}
	return e.shell, nil
}

func (e *RationalEngine) Options() (Options, error) {
	if err := e.check(); err != nil {
		return Options{}, err
	}
	return e.options, nil
}

func (e *RationalEngine) BindCooperation(owner *cooperate.Cooperation) error {
	if err := e.check(); err != nil {
		return err
	}
	return e.context.BindCooperation(owner)
}

func (e *RationalEngine) CollectStatistics(st *statistics.Statistics) error {
	if err := e.check(); err != nil {
		return err
	}
	return e.context.CollectStatistics(st)
}

func (e *RationalEngine) ResetStatistics() error {
	if err := e.check(); err != nil {
		return err
	}
	return e.context.ResetStatistics()
}

func (e *RationalEngine) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
	if err := e.check(); err != nil {
		return err
	}
	selected, err := ReadOptions(pc, p)
	if err != nil {
		return err
	}
	if selected.Engine != EngineMPQ {
		return ErrRationalEngineSelection
	}
	e.options = selected
	return e.context.UpdateParamsRef(pc, p)
}

func (e *RationalEngine) Close() error {
	if e == nil || e.self != e {
		return arena.ErrInvalidSlot
	}
	if e.closed {
		return nil
	}
	if err := e.shell.Close(); err != nil {
		return err
	}
	if err := e.context.Close(); err != nil {
		return err
	}
	e.context = nil
	e.shell = nil
	e.numbers = nil
	e.closed = true
	return nil
}
