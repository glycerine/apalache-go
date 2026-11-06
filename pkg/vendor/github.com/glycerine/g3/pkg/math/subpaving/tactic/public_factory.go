package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:268-289;
// src/math/subpaving/tactic/subpaving_tactic.h:25-29.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	tacticcore "github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/util/params"
)

// NewSubpavingCoreTactic constructs the source selector around its initial concrete numeral branch.
func NewSubpavingCoreTactic(m *ast.ASTManager, c *params.ParamsContext, p *params.ParamsRef) (goaltactic.Tactic, error) {
	if m == nil || c == nil {
		return nil, params.ErrParamsOwner
	}
	if p == nil {
		empty := c.EmptyValue()
		p = &empty
	}
	options, err := ReadOptions(c, p)
	if err != nil {
		return nil, err
	}
	return newSelectedSubpavingTactic(m, c, p, options.Engine)
}

func newConcreteSubpavingTactic(m *ast.ASTManager, c *params.ParamsContext, p *params.ParamsRef, engine EngineKind) (goaltactic.Tactic, error) {
	switch engine {
	case EngineMPQ:
		core, err := NewRationalCoreTacticOwned(m, c, p)
		if err != nil {
			if core != nil {
				err = errors.Join(err, core.Close())
			}
			return nil, err
		}
		return newRationalTacticAdapter(core)
	case EngineMPF:
		core, err := NewArbitraryCoreTacticOwned(m, c, p)
		if err != nil {
			if core != nil {
				err = errors.Join(err, core.Close())
			}
			return nil, err
		}
		return newArbitraryTacticAdapter(core)
	case EngineHWF:
		core, err := NewHardwareCoreTacticOwned(m, c, p)
		if err != nil {
			if core != nil {
				err = errors.Join(err, core.Close())
			}
			return nil, err
		}
		return newHardwareTacticAdapter(core)
	case EngineMPFF:
		core, err := NewFastCoreTacticOwned(m, c, p)
		if err != nil {
			if core != nil {
				err = errors.Join(err, core.Close())
			}
			return nil, err
		}
		return newFastTacticAdapter(core)
	case EngineMPFX:
		core, err := NewFixedCoreTacticOwned(m, c, p)
		if err != nil {
			if core != nil {
				err = errors.Join(err, core.Close())
			}
			return nil, err
		}
		return newFixedTacticAdapter(core)
	default:
		return nil, ErrHardwareEngineSelection
	}
}

// NewSubpavingTactic applies the source's two simplify configurations before
// the selected concrete core tactic.
func NewSubpavingTactic(m *ast.ASTManager, c *params.ParamsContext, g *params.GlobalParamsService, p *params.ParamsRef) (result goaltactic.Tactic, err error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, params.ErrParamsOwner
	}
	if p == nil {
		empty := c.EmptyValue()
		p = &empty
	}
	firstParams, secondParams, err := newPublicSimplifyParams(c, p)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, closePublicSimplifyParams(c, &firstParams, &secondParams)) }()
	first, err := tacticcore.MakeSimplifyTactic(m, c, g, &firstParams)
	if err != nil {
		return nil, err
	}
	if err = first.IncRef(); err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, first.DecRef()) }()
	second, err := tacticcore.MakeSimplifyTactic(m, c, g, &secondParams)
	if err != nil {
		return nil, err
	}
	if err = second.IncRef(); err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, second.DecRef()) }()
	core, err := NewSubpavingCoreTactic(m, c, p)
	if err != nil {
		return nil, err
	}
	if err = core.IncRef(); err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, core.DecRef()) }()
	return goaltactic.NewAndThenTactic(first, second, core)
}

func SubpavingTacticFactory(c *params.ParamsContext, g *params.GlobalParamsService) goaltactic.TacticFactory {
	return goaltactic.TacticFactoryFunc(func(m *ast.ASTManager, p *params.ParamsRef) (goaltactic.Tactic, error) {
		return NewSubpavingTactic(m, c, g, p)
	})
}
