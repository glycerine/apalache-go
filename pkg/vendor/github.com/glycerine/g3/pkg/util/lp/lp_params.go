package lp

// Source: src/util/lp/lp_params.hpp:6-26; src/util/lp/lp_params.pyg.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// LPParams borrows the local reference and acquires the module reference once.
// Getters read those references at the call site, preserving ParamsRef sharing
// and detachment rather than taking a snapshot of their values.
type LPParams struct {
	owner         uparams.ModuleParamsOwner
	self          *LPParams
	frame         *uparams.ModuleParamsFrame
	empty         uparams.ParamsRef
	header        uint64
	ready, closed bool
}

// A nil local selects the source's default empty parameters.
// A nonnil result accompanying an error must be closed.
func NewLPParams(c *uparams.ParamsContext, local *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) (*LPParams, error) {
	if c == nil || g == nil || b == nil || g.CheckOwner(c, b) != nil {
		return nil, uparams.ErrParamsOwner
	}
	n := uint64(reflect.TypeOf(LPParams{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	p := &LPParams{header: n, owner: uparams.ModuleParamsOwner{Budget: b}}
	p.self = p
	p.owner.Self = &p.owner
	if e := p.owner.Bind(c, g); e != nil {
		return p, e
	}
	if local == nil {
		var e error
		p.empty, e = c.NewValue()
		if e != nil {
			return p, e
		}
		local = &p.empty
	}
	p.owner.Running = true
	defer func() { p.owner.Running = false }()
	f, e := p.owner.BeginHelper(local, "lp")
	if e != nil {
		if f != nil {
			e = p.owner.FinishHelper(f, e)
		}
		return p, e
	}
	p.frame = f
	p.ready = true
	return p, nil
}
func (p *LPParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	if !p.ready || p.owner.Running {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (p *LPParams) uintValue(key string, def uint32) (uint32, error) {
	if e := p.open(); e != nil {
		return 0, e
	}
	p.owner.Running = true
	defer func() { p.owner.Running = false }()
	return p.owner.FrameUInt(p.frame, key, def)
}
func (p *LPParams) boolValue(key string, def bool) (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	p.owner.Running = true
	defer func() { p.owner.Running = false }()
	return p.owner.FrameBool(p.frame, key, def)
}
func (p *LPParams) RepFreq() (uint32, error)         { return p.uintValue("rep_freq", 0) }
func (p *LPParams) Min() (bool, error)               { return p.boolValue("min", false) }
func (p *LPParams) PrintStats() (bool, error)        { return p.boolValue("print_stats", false) }
func (p *LPParams) SimplexStrategy() (uint32, error) { return p.uintValue("simplex_strategy", 0) }
func (p *LPParams) BpropOnPivotedRows() (bool, error) {
	return p.boolValue("bprop_on_pivoted_rows", true)
}
func (p *LPParams) Close() error {
	if p == nil || p.self != p {
		return uparams.ErrParamsOwner
	}
	if p.closed {
		return nil
	}
	if p.owner.Running {
		return uparams.ErrParamsPrecondition
	}
	if e := p.owner.CloseHelpers(); e != nil {
		return e
	}
	p.owner.Budget.Release(p.header)
	p.header = 0
	p.frame = nil
	p.closed = true
	return nil
}
