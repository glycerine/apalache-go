package rewriter

// Source: src/ast/rewriter/fpa_rewriter_params.hpp:5-17.

import uparams "github.com/glycerine/g3/pkg/util/params"

// FPARewriterParams is the concrete generated parameter facade. The source
// object borrows p and owns the rewriter-module snapshot in g.
type FPARewriterParams struct {
	self              *FPARewriterParams
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

func NewFPARewriterParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*FPARewriterParams, error) {
	if c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	p := &FPARewriterParams{context: c}
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

func (p *FPARewriterParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}

func (p *FPARewriterParams) HighFPUnspecified() (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	key := "hi_fp_unspecified"
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, false)
}

func (p *FPARewriterParams) Close() error {
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
