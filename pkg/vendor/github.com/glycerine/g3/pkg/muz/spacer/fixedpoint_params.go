package spacer

// Source: src/muz/spacer/spacer_context.cpp and spacer_manager.cpp parameter
// helpers: query-local params_ref values precede the fixedpoint module.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type fixedpointParamView struct {
	context *uparams.ParamsContext
	local   *uparams.ParamsRef
	global  uparams.ParamsRef
}

func newFixedpointParamView(local *uparams.ParamsRef, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*fixedpointParamView, error) {
	global, err := globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return nil, err
	}
	return &fixedpointParamView{context: context, local: local, global: global}, nil
}

func (v *fixedpointParamView) Close() error {
	if v == nil || v.context == nil {
		return nil
	}
	err := v.context.Del(&v.global)
	v.context, v.local = nil, nil
	return err
}

func (v *fixedpointParamView) Bool(name string, fallback bool) (bool, error) {
	if v.local == nil {
		return v.context.GetBoolName(&v.global, &name, fallback)
	}
	return v.context.GetBoolFallbackName(v.local, &name, &v.global, fallback)
}

func (v *fixedpointParamView) UInt(name string, fallback uint32) (uint32, error) {
	if v.local == nil {
		return v.context.GetUIntName(&v.global, &name, fallback)
	}
	return v.context.GetUIntFallbackName(v.local, &name, &v.global, fallback)
}

func (v *fixedpointParamView) Double(name string, fallback float64) (float64, error) {
	if v.local == nil {
		return v.context.GetDoubleName(&v.global, &name, fallback)
	}
	return v.context.GetDoubleFallbackName(v.local, &name, &v.global, fallback)
}
