package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:82-120,193-289.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// selectedSubpavingTactic owns the source imp's current concrete engine and
// parameter snapshot. Updating numeral replaces that engine and converter.
type selectedSubpavingTactic struct {
	self    *selectedSubpavingTactic
	manager *ast.ASTManager
	pc      *params.ParamsContext
	params  params.ParamsRef
	engine  EngineKind
	inner   goaltactic.Tactic
	stats   map[string]float64
	refs    uint32
	closed  bool
}

var _ goaltactic.Tactic = (*selectedSubpavingTactic)(nil)

func newSelectedSubpavingTactic(m *ast.ASTManager, pc *params.ParamsContext, p *params.ParamsRef, engine EngineKind) (*selectedSubpavingTactic, error) {
	if m == nil || pc == nil || p == nil {
		return nil, params.ErrParamsOwner
	}
	a := &selectedSubpavingTactic{manager: m, pc: pc, stats: make(map[string]float64), engine: engine}
	a.self = a
	if err := pc.Check(p); err != nil {
		return nil, err
	}
	owned, err := pc.NewValue()
	if err != nil {
		return nil, err
	}
	a.params = owned
	if err = pc.Assign(&a.params, p); err != nil {
		_ = pc.Del(&a.params)
		return nil, err
	}
	a.inner, err = newConcreteSubpavingTactic(m, pc, &a.params, engine)
	if err != nil {
		_ = pc.Del(&a.params)
		return nil, err
	}
	if err = a.inner.IncRef(); err != nil {
		_ = a.inner.DecRef()
		_ = pc.Del(&a.params)
		return nil, err
	}
	return a, nil
}

func (a *selectedSubpavingTactic) open() error {
	if a == nil || a.self != a {
		return arena.ErrInvalidSlot
	}
	if a.closed {
		return arena.ErrClosed
	}
	return nil
}

func (a *selectedSubpavingTactic) IncRef() error {
	if err := a.open(); err != nil {
		return err
	}
	a.refs++
	return nil
}

func (a *selectedSubpavingTactic) DecRef() error {
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

func (a *selectedSubpavingTactic) Apply(g *goaltactic.Goal, out *goaltactic.TacticResult) error {
	if err := a.open(); err != nil {
		return err
	}
	if a.inner == nil {
		return arena.ErrInvalidSlot
	}
	if err := a.inner.Apply(g, out); err != nil {
		return err
	}
	a.stats = a.inner.CollectStatistics()
	return nil
}

func (a *selectedSubpavingTactic) UpdateParams(p *params.ParamsRef) error {
	if err := a.open(); err != nil {
		return err
	}
	selected, err := ReadOptions(a.pc, p)
	if err != nil {
		return err
	}
	if err = a.pc.Assign(&a.params, p); err != nil {
		return err
	}
	if selected.Engine == a.engine {
		if a.inner == nil {
			return a.installEngine(selected.Engine)
		}
		return a.inner.UpdateParams(&a.params)
	}
	if a.inner != nil {
		if err = a.inner.DecRef(); err != nil {
			return err
		}
		a.inner = nil
	}
	a.engine = selected.Engine
	return a.installEngine(a.engine)
}

func (a *selectedSubpavingTactic) installEngine(engine EngineKind) error {
	inner, err := newConcreteSubpavingTactic(a.manager, a.pc, &a.params, engine)
	if err != nil {
		return err
	}
	if err = inner.IncRef(); err != nil {
		_ = inner.DecRef()
		return err
	}
	a.inner = inner
	return nil
}

func (a *selectedSubpavingTactic) CollectParamDescrs(d *params.ParamDescrs) error {
	if err := a.open(); err != nil {
		return err
	}
	if a.inner == nil {
		return arena.ErrInvalidSlot
	}
	return a.inner.CollectParamDescrs(d)
}

func (a *selectedSubpavingTactic) CollectStatistics() map[string]float64 {
	out := make(map[string]float64, len(a.stats))
	for name, value := range a.stats {
		out[name] = value
	}
	return out
}

func (a *selectedSubpavingTactic) ResetStatistics() error {
	if err := a.open(); err != nil {
		return err
	}
	a.stats = make(map[string]float64)
	return nil
}

func (a *selectedSubpavingTactic) Cleanup() error {
	if err := a.open(); err != nil {
		return err
	}
	if a.inner != nil {
		return a.inner.Cleanup()
	}
	return a.installEngine(a.engine)
}

func (a *selectedSubpavingTactic) Reset() error { return a.Cleanup() }

func (a *selectedSubpavingTactic) SetLogic(logic symbol.Symbol) error {
	if err := a.open(); err != nil {
		return err
	}
	if a.inner == nil {
		return arena.ErrInvalidSlot
	}
	return a.inner.SetLogic(logic)
}

func (a *selectedSubpavingTactic) SetProgressCallback(callback func(uint32)) error {
	if err := a.open(); err != nil {
		return err
	}
	if a.inner == nil {
		return arena.ErrInvalidSlot
	}
	return a.inner.SetProgressCallback(callback)
}

func (a *selectedSubpavingTactic) Translate(m *ast.ASTManager) (goaltactic.Tactic, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	return newSelectedSubpavingTactic(m, a.pc, &a.params, a.engine)
}

func (a *selectedSubpavingTactic) close() error {
	if a == nil || a.self != a {
		return arena.ErrInvalidSlot
	}
	if a.closed {
		return nil
	}
	if a.refs != 0 {
		return arena.ErrBounds
	}
	if a.inner != nil {
		if err := a.inner.DecRef(); err != nil {
			return err
		}
		a.inner = nil
	}
	if err := a.pc.Del(&a.params); err != nil {
		return err
	}
	a.closed = true
	a.manager, a.pc, a.stats = nil, nil, nil
	return nil
}
