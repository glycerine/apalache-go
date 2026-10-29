package interp

// Source: src/interp/interp_params.hpp; src/interp/interp_params.pyg.

import uparams "github.com/glycerine/g3/pkg/util/params"

// Params borrows local parameters and owns the constructor-time interp module
// snapshot used by the generated accessors.
type Params struct {
	self              *Params
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

func NewParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*Params, error) {
	if c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	p := &Params{context: c}
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
	p.global, err = g.SourceGetModuleName("interp")
	if err != nil {
		return nil, err
	}
	return p, nil
}
func (p *Params) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}
func (p *Params) boolean(key string) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, false)
}
func (p *Params) Profile() (bool, error) { return p.boolean("profile") }
func (p *Params) Check() (bool, error)   { return p.boolean("check") }
func (p *Params) Close() error {
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
