package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:193-263;
// concrete Hardware engine adaptation to src/tactic/tactic.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type hardwareTacticAdapter struct {
	self   *hardwareTacticAdapter
	core   *HardwareCoreTactic
	refs   uint32
	closed bool
}

var _ goaltactic.Tactic = (*hardwareTacticAdapter)(nil)

func newHardwareTacticAdapter(core *HardwareCoreTactic) (*hardwareTacticAdapter, error) {
	if core == nil || core.open() != nil {
		return nil, arena.ErrInvalidSlot
	}
	a := &hardwareTacticAdapter{core: core}
	a.self = a
	return a, nil
}
func (a *hardwareTacticAdapter) open() error {
	if a == nil || a.self != a {
		return arena.ErrInvalidSlot
	}
	if a.closed || a.core == nil {
		return arena.ErrClosed
	}
	return a.core.open()
}
func (a *hardwareTacticAdapter) IncRef() error {
	if err := a.open(); err != nil {
		return err
	}
	a.refs++
	return nil
}
func (a *hardwareTacticAdapter) DecRef() error {
	if err := a.open(); err != nil {
		return err
	}
	if a.refs == 0 {
		return arena.ErrBounds
	}
	a.refs--
	if a.refs == 0 {
		return a.close()
	}
	return nil
}
func (a *hardwareTacticAdapter) Apply(g *goaltactic.Goal, out *goaltactic.TacticResult) error {
	if err := a.open(); err != nil {
		return err
	}
	if g == nil || out == nil {
		return ast.ErrASTHandle
	}
	if err := out.Prepare(g.Manager()); err != nil {
		return err
	}
	result, err := a.core.Apply(g)
	if err != nil {
		return err
	}
	return out.AddGoal(result)
}
func (a *hardwareTacticAdapter) UpdateParams(p *params.ParamsRef) error {
	if err := a.open(); err != nil {
		return err
	}
	return a.core.UpdateParams(p)
}
func (a *hardwareTacticAdapter) CollectParamDescrs(d *params.ParamDescrs) error {
	if err := a.open(); err != nil {
		return err
	}
	return CollectParamDescriptions(d)
}
func (a *hardwareTacticAdapter) CollectStatistics() map[string]float64 {
	out := map[string]float64{}
	if a.open() != nil {
		return out
	}
	s := statistics.New()
	if a.core.CollectStatistics(s) != nil {
		return out
	}
	for i := 0; i < s.Size(); i++ {
		if s.IsUint(i) {
			out[string(s.Key(i))] += float64(s.UintValue(i))
		} else {
			out[string(s.Key(i))] += s.DoubleValue(i)
		}
	}
	return out
}
func (a *hardwareTacticAdapter) ResetStatistics() error {
	if err := a.open(); err != nil {
		return err
	}
	return a.core.ResetStatistics()
}
func (a *hardwareTacticAdapter) Cleanup() error {
	if err := a.open(); err != nil {
		return err
	}
	return a.core.Cleanup()
}
func (a *hardwareTacticAdapter) Reset() error                           { return a.Cleanup() }
func (a *hardwareTacticAdapter) SetLogic(symbol.Symbol) error           { return a.open() }
func (a *hardwareTacticAdapter) SetProgressCallback(func(uint32)) error { return a.open() }
func (a *hardwareTacticAdapter) Translate(m *ast.ASTManager) (goaltactic.Tactic, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	var core *HardwareCoreTactic
	var err error
	if a.core.ownsNumbers {
		core, err = NewHardwareCoreTacticOwned(m, a.core.pc, &a.core.params)
	} else {
		core, err = NewHardwareCoreTactic(m, a.core.numbers, a.core.pc, &a.core.params)
	}
	if err != nil {
		return nil, err
	}
	return newHardwareTacticAdapter(core)
}
func (a *hardwareTacticAdapter) close() error {
	if a == nil || a.self != a {
		return arena.ErrInvalidSlot
	}
	if a.closed {
		return nil
	}
	if a.refs != 0 {
		return arena.ErrBounds
	}
	if err := a.core.Close(); err != nil {
		return err
	}
	a.core = nil
	a.closed = true
	return nil
}
