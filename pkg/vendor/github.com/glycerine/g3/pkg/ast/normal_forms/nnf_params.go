package normal_forms

// Source: src/ast/normal_forms/nnf_params.hpp:5-27.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// NNFParams is the concrete generated parameter facade. The source object
// borrows p and owns the "nnf" module snapshot captured by its constructor.
type NNFParams struct {
	self              *NNFParams
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

func NewNNFParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*NNFParams, error) {
	if c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	p := &NNFParams{context: c}
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
	p.global, e = g.SourceGetModuleName("nnf")
	if e != nil {
		return nil, e
	}
	return p, nil
}

func (p *NNFParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}

func (p *NNFParams) MaxMemory() (uint32, error) {
	if e := p.open(); e != nil {
		return 0, e
	}
	key := "max_memory"
	return p.context.GetUIntFallbackName(p.local, &key, &p.global, ^uint32(0))
}

func (p *NNFParams) SkHack() (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	key := "sk_hack"
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, false)
}

func (p *NNFParams) Mode() (usymbol.Symbol, error) {
	if e := p.open(); e != nil {
		return usymbol.Symbol{}, e
	}
	// The generated C++ constructs symbol("skolem") before entering get_sym,
	// including when a local or module value wins.
	d, e := p.context.SymbolsField.Intern("skolem")
	if e != nil {
		return usymbol.Symbol{}, e
	}
	key := "mode"
	return p.context.GetSymbolFallbackName(p.local, &key, &p.global, d)
}

func (p *NNFParams) IgnoreLabels() (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	key := "ignore_labels"
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, false)
}

func (p *NNFParams) Skolemize() (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	key := "skolemize"
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, true)
}

func (p *NNFParams) Close() error {
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
