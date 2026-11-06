package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:193-263;
// concrete Fast engine adaptation to src/tactic/tactic.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type fastTacticAdapter struct {
	self   *fastTacticAdapter
	core   *FastCoreTactic
	refs   uint32
	closed bool
}

var _ goaltactic.Tactic = (*fastTacticAdapter)(nil)

func newFastTacticAdapter(core *FastCoreTactic) (*fastTacticAdapter, error) {
	if core == nil || core.open() != nil {
		return nil, arena.ErrInvalidSlot
	}
	a := &fastTacticAdapter{core: core}
	a.self = a
	return a, nil
}
func (a *fastTacticAdapter) open() error {
	if a == nil || a.self != a {
		return arena.ErrInvalidSlot
	}
	if a.closed || a.core == nil {
		return arena.ErrClosed
	}
	return a.core.open()
}
func (a *fastTacticAdapter) IncRef() error {
	if err := a.open(); err != nil {
		return err
	}
	a.refs++
	return nil
}
func (a *fastTacticAdapter) DecRef() error {
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
func (a *fastTacticAdapter) Apply(g *goaltactic.Goal, out *goaltactic.TacticResult) error {
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
func (a *fastTacticAdapter) UpdateParams(p *params.ParamsRef) error {
	if err := a.open(); err != nil {
		return err
	}
	return a.core.UpdateParams(p)
}
func (a *fastTacticAdapter) CollectParamDescrs(d *params.ParamDescrs) error {
	if err := a.open(); err != nil {
		return err
	}
	return CollectParamDescriptions(d)
}
func (a *fastTacticAdapter) CollectStatistics() map[string]float64 {
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
func (a *fastTacticAdapter) ResetStatistics() error {
	if err := a.open(); err != nil {
		return err
	}
	return a.core.ResetStatistics()
}
func (a *fastTacticAdapter) Cleanup() error {
	if err := a.open(); err != nil {
		return err
	}
	return a.core.Cleanup()
}
func (a *fastTacticAdapter) Reset() error                           { return a.Cleanup() }
func (a *fastTacticAdapter) SetLogic(symbol.Symbol) error           { return a.open() }
func (a *fastTacticAdapter) SetProgressCallback(func(uint32)) error { return a.open() }
func (a *fastTacticAdapter) Translate(m *ast.ASTManager) (goaltactic.Tactic, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	var core *FastCoreTactic
	var err error
	if a.core.ownsNumerals {
		core, err = NewFastCoreTacticOwned(m, a.core.pc, &a.core.params)
	} else {
		core, err = NewFastCoreTactic(m, a.core.numbers, a.core.rational, a.core.pc, &a.core.params)
	}
	if err != nil {
		return nil, err
	}
	return newFastTacticAdapter(core)
}
func (a *fastTacticAdapter) close() error {
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
