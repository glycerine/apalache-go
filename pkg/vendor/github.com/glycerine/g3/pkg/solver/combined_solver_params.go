package solver

// Source: src/solver/combined_solver_params.hpp;
// src/solver/combined_solver_params.pyg.

import uparams "github.com/glycerine/g3/pkg/util/params"

// CombinedSolverParams is the generated parameter facade. It borrows the
// caller's local value and owns the constructor-time combined_solver module
// snapshot.
type CombinedSolverParams struct {
	self              *CombinedSolverParams
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

func NewCombinedSolverParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*CombinedSolverParams, error) {
	if c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	p := &CombinedSolverParams{context: c}
	p.self = p
	if local == nil {
		p.empty = c.EmptyValue()
		local = &p.empty
	}
	if err := c.Check(local); err != nil {
		return nil, err
	}
	p.local = local
	var err error
	p.global, err = g.SourceGetModuleName("combined_solver")
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *CombinedSolverParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}
func (p *CombinedSolverParams) Solver2Timeout() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	key := "solver2_timeout"
	return p.context.GetUIntFallbackName(p.local, &key, &p.global, ^uint32(0))
}
func (p *CombinedSolverParams) IgnoreSolver1() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	key := "ignore_solver1"
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, false)
}
func (p *CombinedSolverParams) Solver2Unknown() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	key := "solver2_unknown"
	return p.context.GetUIntFallbackName(p.local, &key, &p.global, 1)
}
func (p *CombinedSolverParams) Close() error {
	if p == nil || p.self != p {
		return uparams.ErrParamsOwner
	}
	if p.closed {
		return nil
	}
	if p.destructorStarted {
		if err := p.context.CompleteDecrement(&p.global, p.decrement); err != nil {
			return err
		}
	} else {
		p.destructorStarted = true
		if err := p.context.DelTracked(&p.global, &p.decrement); err != nil {
			return err
		}
	}
	p.closed = true
	return nil
}

func CollectCombinedSolverParamDescrs(d *uparams.ParamDescrs) error {
	if d == nil {
		return uparams.ErrParamsOwner
	}
	return catalogCollectCombinedSolverParams(d)
}
