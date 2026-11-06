package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:92-117, selected
// MPFX branch; src/math/subpaving/subpaving.cpp:279-280.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/subpaving"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
)

var ErrFixedEngineSelection = errors.New("subpaving tactic: selected numeral engine is not fixed point")

// FixedEngine owns the selected MPFX shell and context. Both numeral managers
// and the resource limit remain borrowed from the tactic owner.
type FixedEngine struct {
	self    *FixedEngine
	shell   *subpaving.FixedShellCoefficients
	numbers *numeral.FixedPointManager
	options Options
	closed  bool
}

func NewFixedEngine(limit *control.Reslimit, numbers *numeral.FixedPointManager, rational *numeral.RationalManager, pc *params.ParamsContext, p *params.ParamsRef) (*FixedEngine, error) {
	if limit == nil || numbers == nil || rational == nil {
		return nil, arena.ErrInvalidSlot
	}
	selected, err := ReadOptions(pc, p)
	if err != nil {
		return nil, err
	}
	if selected.Engine != EngineMPFX {
		return nil, ErrFixedEngineSelection
	}
	shell, err := subpaving.NewFixedShellFromParams(numbers, limit, rational, pc, p)
	if err != nil {
		return nil, err
	}
	e := &FixedEngine{shell: shell, numbers: numbers, options: selected}
	e.self = e
	return e, nil
}

func (e *FixedEngine) check() error {
	if e == nil || e.self != e {
		return arena.ErrInvalidSlot
	}
	if e.closed {
		return arena.ErrClosed
	}
	if e.shell == nil || e.numbers == nil {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (e *FixedEngine) Shell() (*subpaving.FixedShellCoefficients, error) {
	if err := e.check(); err != nil {
		return nil, err
	}
	return e.shell, nil
}

func (e *FixedEngine) Options() (Options, error) {
	if err := e.check(); err != nil {
		return Options{}, err
	}
	return e.options, nil
}

func (e *FixedEngine) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
	if err := e.check(); err != nil {
		return err
	}
	selected, err := ReadOptions(pc, p)
	if err != nil {
		return err
	}
	if selected.Engine != EngineMPFX {
		return ErrFixedEngineSelection
	}
	e.options = selected
	return e.shell.UpdateParamsRef(pc, p)
}

func (e *FixedEngine) Search() error {
	if err := e.check(); err != nil {
		return err
	}
	return e.shell.Search()
}

func (e *FixedEngine) BindCooperation(owner *cooperate.Cooperation) error {
	if err := e.check(); err != nil {
		return err
	}
	return e.shell.BindCooperation(owner)
}

func (e *FixedEngine) Close() error {
	if e == nil || e.self != e {
		return arena.ErrInvalidSlot
	}
	if e.closed {
		return nil
	}
	if err := e.shell.Close(); err != nil {
		return err
	}
	e.shell = nil
	e.numbers = nil
	e.closed = true
	return nil
}
