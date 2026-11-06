package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:92-120,
// concrete HWF branch and src/math/subpaving/subpaving.cpp:271-273.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/subpaving"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
)

var ErrHardwareEngineSelection = errors.New("subpaving tactic: selected numeral engine is not hardware float")

// HardwareEngine owns the selected HWF context. Its rational manager and
// resource limit belong to the caller; the concrete tactic owns this context
// alongside its expression converter.
type HardwareEngine struct {
	self    *HardwareEngine
	context *subpaving.HardwareContext
	shell   *subpaving.HardwareShellCoefficients
	qm      *numeral.RationalManager
	options Options
	closed  bool
}

func NewHardwareEngine(limit *control.Reslimit, qm *numeral.RationalManager, pc *params.ParamsContext, p *params.ParamsRef) (*HardwareEngine, error) {
	if limit == nil || qm == nil || qm.Z == nil {
		return nil, arena.ErrInvalidSlot
	}
	selected, err := ReadOptions(pc, p)
	if err != nil {
		return nil, err
	}
	if selected.Engine != EngineHWF {
		return nil, ErrHardwareEngineSelection
	}
	m, err := numeral.NewHardwareFloatNumeralManager(numeral.NewHardwareFloatManager(), 11, 53)
	if err != nil {
		return nil, err
	}
	c, err := subpaving.NewHardwareContextFromParams(m, limit, pc, p)
	if c == nil {
		return nil, err
	}
	e := &HardwareEngine{context: c, qm: qm, options: selected}
	e.self = e
	if runtime := qm.RuntimeOwner(); runtime != nil {
		if process := runtime.ProcessOwner(); process != nil {
			if bindErr := c.BindProcessAccounting(process); bindErr != nil {
				return e, errors.Join(err, bindErr)
			}
		}
	}
	if err != nil {
		return e, err
	}
	e.shell, err = subpaving.NewHardwareShellCoefficients(c, qm)
	return e, err
}

func (e *HardwareEngine) check() error {
	if e == nil || e.self != e {
		return arena.ErrInvalidSlot
	}
	if e.closed {
		return arena.ErrClosed
	}
	if e.context == nil {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (e *HardwareEngine) Shell() (*subpaving.HardwareShellCoefficients, error) {
	if err := e.check(); err != nil {
		return nil, err
	}
	if e.shell == nil {
		return nil, arena.ErrInvalidSlot
	}
	return e.shell, nil
}

func (e *HardwareEngine) BindCooperation(owner *cooperate.Cooperation) error {
	if err := e.check(); err != nil {
		return err
	}
	return e.shell.BindCooperation(owner)
}

func (e *HardwareEngine) Options() (Options, error) {
	if err := e.check(); err != nil {
		return Options{}, err
	}
	return e.options, nil
}

func (e *HardwareEngine) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
	if err := e.check(); err != nil {
		return err
	}
	selected, err := ReadOptions(pc, p)
	if err != nil {
		return err
	}
	if selected.Engine != EngineHWF {
		return ErrHardwareEngineSelection
	}
	e.options = selected
	return e.shell.UpdateParamsRef(pc, p)
}

func (e *HardwareEngine) Close() error {
	if e == nil || e.self != e {
		return arena.ErrInvalidSlot
	}
	if e.closed {
		return nil
	}
	if err := e.context.Close(); err != nil {
		return err
	}
	e.shell = nil
	e.context = nil
	e.qm = nil
	e.closed = true
	return nil
}
