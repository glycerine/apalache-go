package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:92-120, selected MPF
// branch and src/math/subpaving/subpaving.cpp:102-107,271-273.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/subpaving"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
)

var ErrArbitraryEngineSelection = errors.New("subpaving tactic: selected numeral engine is not arbitrary float")

// ArbitraryEngine owns the selected MPF shell and context. The concrete
// numeral manager and resource limit are borrowed from the tactic's owner.
type ArbitraryEngine struct {
	self    *ArbitraryEngine
	shell   *subpaving.ArbitraryShellCoefficients
	numbers *numeral.ArbitraryFloatNumeralManager
	options Options
	closed  bool
}

func NewArbitraryEngine(limit *control.Reslimit, numbers *numeral.ArbitraryFloatNumeralManager, pc *params.ParamsContext, p *params.ParamsRef) (*ArbitraryEngine, error) {
	if limit == nil || numbers == nil {
		return nil, arena.ErrInvalidSlot
	}
	selected, err := ReadOptions(pc, p)
	if err != nil {
		return nil, err
	}
	if selected.Engine != EngineMPF {
		return nil, ErrArbitraryEngineSelection
	}
	shell, cause := subpaving.NewArbitraryShellFromParams(numbers, limit, pc, p)
	if shell == nil {
		return nil, cause
	}
	e := &ArbitraryEngine{shell: shell, numbers: numbers, options: selected}
	e.self = e
	return e, cause
}

func (e *ArbitraryEngine) check() error {
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

func (e *ArbitraryEngine) Shell() (*subpaving.ArbitraryShellCoefficients, error) {
	if err := e.check(); err != nil {
		return nil, err
	}
	return e.shell, nil
}

func (e *ArbitraryEngine) BindCooperation(owner *cooperate.Cooperation) error {
	if err := e.check(); err != nil {
		return err
	}
	return e.shell.BindCooperation(owner)
}

func (e *ArbitraryEngine) Options() (Options, error) {
	if err := e.check(); err != nil {
		return Options{}, err
	}
	return e.options, nil
}

func (e *ArbitraryEngine) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
	if err := e.check(); err != nil {
		return err
	}
	selected, err := ReadOptions(pc, p)
	if err != nil {
		return err
	}
	if selected.Engine != EngineMPF {
		return ErrArbitraryEngineSelection
	}
	e.options = selected
	return e.shell.UpdateParamsRef(pc, p)
}

func (e *ArbitraryEngine) Close() error {
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
