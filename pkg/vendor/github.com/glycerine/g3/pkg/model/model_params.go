package model

// Source: src/model/model_params.hpp; src/model/model_params.pyg.

import uparams "github.com/glycerine/g3/pkg/util/params"

// ModelParams is the generated model parameter facade. It borrows the local
// value and owns the constructor-time global model-module snapshot.
type ModelParams struct {
	self              *ModelParams
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

func NewModelParams(context *uparams.ParamsContext, globals *uparams.GlobalParamsService, local *uparams.ParamsRef) (*ModelParams, error) {
	if context == nil || globals == nil || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	p := &ModelParams{context: context}
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
	p.global, err = globals.SourceGetModuleName("model")
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *ModelParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}

func (p *ModelParams) boolean(key string) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, false)
}

func (p *ModelParams) Partial() (bool, error)    { return p.boolean("partial") }
func (p *ModelParams) V1() (bool, error)         { return p.boolean("v1") }
func (p *ModelParams) V2() (bool, error)         { return p.boolean("v2") }
func (p *ModelParams) Compact() (bool, error)    { return p.boolean("compact") }
func (p *ModelParams) Completion() (bool, error) { return p.boolean("completion") }

func (p *ModelParams) Close() error {
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
