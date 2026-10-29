package util

import uparams "github.com/glycerine/g3/pkg/util/params"

type ParserParams struct {
	self                      *ParserParams
	context                   *uparams.ParamsContext
	local                     *uparams.ParamsRef
	empty, global             uparams.ParamsRef
	receipt                   uparams.ParamsDecrementReceipt
	destructorStarted, closed bool
}

func NewParserParams(context *uparams.ParamsContext, globals *uparams.GlobalParamsService, local *uparams.ParamsRef) (*ParserParams, error) {
	if context == nil || globals == nil || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	p := &ParserParams{context: context}
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
	p.global, err = globals.SourceGetModuleName("parser")
	if err != nil {
		return nil, err
	}
	return p, nil
}
func (p *ParserParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}
func (p *ParserParams) boolean(key string, fallback bool) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, fallback)
}
func (p *ParserParams) IgnoreUserPatterns() (bool, error) {
	return p.boolean("ignore_user_patterns", false)
}
func (p *ParserParams) IgnoreBadPatterns() (bool, error) {
	return p.boolean("ignore_bad_patterns", true)
}
func (p *ParserParams) ErrorForVisualStudio() (bool, error) {
	return p.boolean("error_for_visual_studio", false)
}
func (p *ParserParams) Close() error {
	if p == nil || p.self != p {
		return uparams.ErrParamsOwner
	}
	if p.closed {
		return nil
	}
	if p.destructorStarted {
		if err := p.context.CompleteDecrement(&p.global, p.receipt); err != nil {
			return err
		}
	} else {
		p.destructorStarted = true
		if err := p.context.DelTracked(&p.global, &p.receipt); err != nil {
			return err
		}
	}
	p.closed = true
	return nil
}
