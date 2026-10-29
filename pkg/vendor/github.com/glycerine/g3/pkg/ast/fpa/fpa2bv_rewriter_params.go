package fpa

// Source: src/ast/fpa/fpa2bv_rewriter_params.hpp:5-17.

import uparams "github.com/glycerine/g3/pkg/util/params"

// FPA2BVRewriterParams is the concrete generated parameter facade. Local is a
// borrowed params_ref; global is the owned "rewriter" module snapshot captured
// by the source constructor.
type FPA2BVRewriterParams struct {
	self              *FPA2BVRewriterParams
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

func NewFPA2BVRewriterParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*FPA2BVRewriterParams, error) {
	if c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	p := &FPA2BVRewriterParams{context: c}
	p.self = p
	if local == nil {
		p.empty = c.EmptyValue()
		local = &p.empty
	}
	if e := c.Check(local); e != nil {
		return nil, e
	}
	p.local = local
	var e error
	p.global, e = g.SourceGetModuleName("rewriter")
	if e != nil {
		return nil, e
	}
	return p, nil
}

func (p *FPA2BVRewriterParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}

func (p *FPA2BVRewriterParams) HighFPUnspecified() (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	key := "hi_fp_unspecified"
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, false)
}

func (p *FPA2BVRewriterParams) Close() error {
	if p == nil || p.self != p {
		return uparams.ErrParamsOwner
	}
	if p.closed {
		return nil
	}
	if p.destructorStarted {
		if e := p.context.CompleteDecrement(&p.global, p.decrement); e != nil {
			return e
		}
	} else {
		p.destructorStarted = true
		if e := p.context.DelTracked(&p.global, &p.decrement); e != nil {
			return e
		}
	}
	p.closed = true
	return nil
}
