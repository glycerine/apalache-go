package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:193-263;
// concrete Arbitrary engine adaptation to src/tactic/tactic.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type arbitraryTacticAdapter struct {
	self   *arbitraryTacticAdapter
	core   *ArbitraryCoreTactic
	refs   uint32
	closed bool
}

var _ goaltactic.Tactic = (*arbitraryTacticAdapter)(nil)

func newArbitraryTacticAdapter(core *ArbitraryCoreTactic) (*arbitraryTacticAdapter, error) {
	if core == nil || core.open() != nil {
		return nil, arena.ErrInvalidSlot
	}
	a := &arbitraryTacticAdapter{core: core}
	a.self = a
	return a, nil
}
func (a *arbitraryTacticAdapter) open() error {
	if a == nil || a.self != a {
		return arena.ErrInvalidSlot
	}
	if a.closed || a.core == nil {
		return arena.ErrClosed
	}
	return a.core.open()
}
func (a *arbitraryTacticAdapter) IncRef() error {
	if err := a.open(); err != nil {
		return err
	}
	a.refs++
	return nil
}
func (a *arbitraryTacticAdapter) DecRef() error {
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
func (a *arbitraryTacticAdapter) Apply(g *goaltactic.Goal, out *goaltactic.TacticResult) error {
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
func (a *arbitraryTacticAdapter) UpdateParams(p *params.ParamsRef) error {
	if err := a.open(); err != nil {
		return err
	}
	return a.core.UpdateParams(p)
}
func (a *arbitraryTacticAdapter) CollectParamDescrs(d *params.ParamDescrs) error {
	if err := a.open(); err != nil {
		return err
	}
	return CollectParamDescriptions(d)
}
func (a *arbitraryTacticAdapter) CollectStatistics() map[string]float64 {
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
func (a *arbitraryTacticAdapter) ResetStatistics() error {
	if err := a.open(); err != nil {
		return err
	}
	return a.core.ResetStatistics()
}
func (a *arbitraryTacticAdapter) Cleanup() error {
	if err := a.open(); err != nil {
		return err
	}
	return a.core.Cleanup()
}
func (a *arbitraryTacticAdapter) Reset() error                           { return a.Cleanup() }
func (a *arbitraryTacticAdapter) SetLogic(symbol.Symbol) error           { return a.open() }
func (a *arbitraryTacticAdapter) SetProgressCallback(func(uint32)) error { return a.open() }
func (a *arbitraryTacticAdapter) Translate(m *ast.ASTManager) (goaltactic.Tactic, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	var core *ArbitraryCoreTactic
	var err error
	if a.core.ownsNumbers {
		core, err = NewArbitraryCoreTacticOwned(m, a.core.pc, &a.core.params)
	} else {
		core, err = NewArbitraryCoreTactic(m, a.core.numbers, a.core.rational, a.core.pc, &a.core.params)
	}
	if err != nil {
		return nil, err
	}
	return newArbitraryTacticAdapter(core)
}
func (a *arbitraryTacticAdapter) close() error {
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
