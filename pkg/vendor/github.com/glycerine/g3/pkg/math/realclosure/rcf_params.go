package realclosure

// Source: src/math/realclosure/rcf_params.pyg and generated
// src/math/realclosure/rcf_params.hpp.

import uparams "github.com/glycerine/g3/pkg/util/params"

// RCFParams is the generated real-closed-field parameter facade. It borrows
// the caller's local parameters and owns the constructor-time rcf module
// reference used as the second lookup source by every getter.
type RCFParams struct {
	self              *RCFParams
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

// NewRCFParams preserves the generated constructor's local-reference borrow
// and gparams::get_module("rcf") acquisition. A nil local selects the source
// empty parameters.
func NewRCFParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*RCFParams, error) {
	if c == nil || g == nil || g.CheckOwner(c, g.Config.Budget) != nil {
		return nil, uparams.ErrParamsOwner
	}
	p := &RCFParams{context: c}
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
	p.global, err = g.SourceGetModuleName("rcf")
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *RCFParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}

func (p *RCFParams) boolean(key string, def bool) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, def)
}

func (p *RCFParams) unsigned(key string, def uint32) (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.context.GetUIntFallbackName(p.local, &key, &p.global, def)
}

func (p *RCFParams) UsePrem() (bool, error) { return p.boolean("use_prem", true) }

func (p *RCFParams) CleanDenominators() (bool, error) {
	return p.boolean("clean_denominators", true)
}

func (p *RCFParams) InitialPrecision() (uint32, error) {
	return p.unsigned("initial_precision", 24)
}

func (p *RCFParams) InfPrecision() (uint32, error) {
	return p.unsigned("inf_precision", 24)
}

func (p *RCFParams) MaxPrecision() (uint32, error) {
	return p.unsigned("max_precision", 128)
}

func (p *RCFParams) LazyAlgebraicNormalization() (bool, error) {
	return p.boolean("lazy_algebraic_normalization", true)
}

// Close executes the owned params_ref destructor. A stopped tracked decrement
// remains resumable on the next call.
func (p *RCFParams) Close() error {
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
