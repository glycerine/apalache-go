package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:92-120, selected
// MPFF branch; src/math/subpaving/subpaving.cpp:260,275-276.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/subpaving"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
)

var ErrFastEngineSelection = errors.New("subpaving tactic: selected numeral engine is not fast float")

// FastEngine owns the selected MPFF shell and context. The numeral and
// rational managers, parameter context and resource limit remain borrowed.
type FastEngine struct {
	self    *FastEngine
	shell   *subpaving.FastShellCoefficients
	numbers *numeral.FastFloatManager
	options Options
	closed  bool
}

func NewFastEngine(limit *control.Reslimit, numbers *numeral.FastFloatManager, rational *numeral.RationalManager, pc *params.ParamsContext, p *params.ParamsRef) (*FastEngine, error) {
	if limit == nil || numbers == nil || rational == nil {
		return nil, arena.ErrInvalidSlot
	}
	selected, err := ReadOptions(pc, p)
	if err != nil {
		return nil, err
	}
	if selected.Engine != EngineMPFF {
		return nil, ErrFastEngineSelection
	}
	fastRuntime, rationalRuntime := numbers.RuntimeOwner(), rational.RuntimeOwner()
	if fastRuntime == nil || rationalRuntime == nil {
		return nil, arena.ErrInvalidSlot
	}
	if fastProcess, rationalProcess := fastRuntime.ProcessOwner(), rationalRuntime.ProcessOwner(); (fastProcess != nil || rationalProcess != nil) && fastProcess != rationalProcess {
		return nil, arena.ErrConfig
	}
	shell, cause := subpaving.NewFastShellFromParams(numbers, limit, rational, pc, p)
	if shell == nil {
		return nil, cause
	}
	e := &FastEngine{shell: shell, numbers: numbers, options: selected}
	e.self = e
	return e, cause
}

func (e *FastEngine) check() error {
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

func (e *FastEngine) Shell() (*subpaving.FastShellCoefficients, error) {
	if err := e.check(); err != nil {
		return nil, err
	}
	return e.shell, nil
}

// BindCooperation borrows the process's source checkpoint owner.
func (e *FastEngine) BindCooperation(owner *cooperate.Cooperation) error {
	if err := e.check(); err != nil {
		return err
	}
	return e.shell.BindCooperation(owner)
}

func (e *FastEngine) Options() (Options, error) {
	if err := e.check(); err != nil {
		return Options{}, err
	}
	return e.options, nil
}

// UpdateParamsRef follows the selected MPFF branch's display flag update,
// then forwards source-order scalar updates into its existing context.
func (e *FastEngine) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
	if err := e.check(); err != nil {
		return err
	}
	selected, err := ReadOptions(pc, p)
	if err != nil {
		return err
	}
	if selected.Engine != EngineMPFF {
		return ErrFastEngineSelection
	}
	e.options = selected
	return e.shell.UpdateParamsRef(pc, p)
}

func (e *FastEngine) Close() error {
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
