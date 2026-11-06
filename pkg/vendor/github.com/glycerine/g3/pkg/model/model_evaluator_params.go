package model

// Source: src/model/model_evaluator_params.hpp;
// src/model/model_evaluator_params.pyg.

import uparams "github.com/glycerine/g3/pkg/util/params"

// ModelEvaluatorParams is the generated evaluator parameter facade. It
// borrows the local value and owns the global model_evaluator snapshot.
type ModelEvaluatorParams struct {
	self              *ModelEvaluatorParams
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

func NewModelEvaluatorParams(context *uparams.ParamsContext, globals *uparams.GlobalParamsService, local *uparams.ParamsRef) (*ModelEvaluatorParams, error) {
	if context == nil || globals == nil || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	p := &ModelEvaluatorParams{context: context}
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
	p.global, err = globals.SourceGetModuleName("model_evaluator")
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *ModelEvaluatorParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}

func (p *ModelEvaluatorParams) unsigned(key string, fallback uint32) (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.context.GetUIntFallbackName(p.local, &key, &p.global, fallback)
}

func (p *ModelEvaluatorParams) boolean(key string, fallback bool) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, fallback)
}

func (p *ModelEvaluatorParams) MaxMemory() (uint32, error) {
	return p.unsigned("max_memory", ^uint32(0))
}
func (p *ModelEvaluatorParams) MaxSteps() (uint32, error) {
	return p.unsigned("max_steps", ^uint32(0))
}
func (p *ModelEvaluatorParams) Completion() (bool, error) {
	return p.boolean("completion", false)
}
func (p *ModelEvaluatorParams) Cache() (bool, error) { return p.boolean("cache", true) }
func (p *ModelEvaluatorParams) ArrayEqualities() (bool, error) {
	return p.boolean("array_equalities", true)
}

func (p *ModelEvaluatorParams) Close() error {
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
