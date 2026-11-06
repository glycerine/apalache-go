package sls

// Source: src/tactic/sls/sls_params.pyg and its generated sls_params.hpp.

import uparams "github.com/glycerine/g3/pkg/util/params"

// Params is the concrete generated SLS parameter facade. It borrows the local
// value and owns the constructor-time global sls-module snapshot.
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

func NewParams(context *uparams.ParamsContext, globals *uparams.GlobalParamsService, local *uparams.ParamsRef) (*Params, error) {
	if context == nil || globals == nil || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	p := &Params{context: context}
	p.self = p
	if local == nil {
		p.empty = context.EmptyValue()
		local = &p.empty
	}
	if err := context.Check(local); err != nil {
		return nil, err
	}
	p.local = local
	var err error
	p.global, err = globals.SourceGetModuleName("sls")
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

func (p *Params) boolean(key string, def bool) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, def)
}

func (p *Params) unsigned(key string, def uint32) (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.context.GetUIntFallbackName(p.local, &key, &p.global, def)
}

func (p *Params) floating(key string, def float64) (float64, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.context.GetDoubleFallbackName(p.local, &key, &p.global, def)
}

func (p *Params) MaxMemory() (uint32, error)   { return p.unsigned("max_memory", ^uint32(0)) }
func (p *Params) MaxRestarts() (uint32, error) { return p.unsigned("max_restarts", ^uint32(0)) }
func (p *Params) Walksat() (bool, error)       { return p.boolean("walksat", true) }
func (p *Params) WalksatUCB() (bool, error)    { return p.boolean("walksat_ucb", true) }
func (p *Params) WalksatUCBConstant() (float64, error) {
	return p.floating("walksat_ucb_constant", 20.0)
}
func (p *Params) WalksatUCBInit() (bool, error) { return p.boolean("walksat_ucb_init", false) }
func (p *Params) WalksatUCBForget() (float64, error) {
	return p.floating("walksat_ucb_forget", 1.0)
}
func (p *Params) WalksatUCBNoise() (float64, error) {
	return p.floating("walksat_ucb_noise", 0.0002)
}
func (p *Params) WalksatRepick() (bool, error) { return p.boolean("walksat_repick", true) }
func (p *Params) ScaleUnsat() (float64, error) { return p.floating("scale_unsat", 0.5) }
func (p *Params) PawsInit() (uint32, error)    { return p.unsigned("paws_init", 40) }
func (p *Params) PawsSP() (uint32, error)      { return p.unsigned("paws_sp", 52) }
func (p *Params) WP() (uint32, error)          { return p.unsigned("wp", 100) }
func (p *Params) VNSMC() (uint32, error)       { return p.unsigned("vns_mc", 0) }
func (p *Params) VNSRepick() (bool, error)     { return p.boolean("vns_repick", false) }
func (p *Params) RestartBase() (uint32, error) { return p.unsigned("restart_base", 100) }
func (p *Params) RestartInit() (bool, error)   { return p.boolean("restart_init", false) }
func (p *Params) EarlyPrune() (bool, error)    { return p.boolean("early_prune", true) }
func (p *Params) RandomOffset() (bool, error)  { return p.boolean("random_offset", true) }
func (p *Params) Rescore() (bool, error)       { return p.boolean("rescore", true) }
func (p *Params) TrackUnsat() (bool, error)    { return p.boolean("track_unsat", false) }
func (p *Params) RandomSeed() (uint32, error)  { return p.unsigned("random_seed", 0) }

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
