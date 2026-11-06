package pattern

// Source: src/ast/pattern/pattern_inference_params.h:24-50 and
// src/ast/pattern/pattern_inference_params_helper.hpp:5-34.

import "github.com/glycerine/g3/pkg/util/diagnostic"

// InferenceOptions is the concrete scalar state consumed by the pattern
// rewriter. Arithmetic preserves the source unsigned-to-enum cast, including
// values outside the three named modes.
type InferenceOptions struct {
	MaxMultiPatterns          uint32
	BlockLoopPatterns         bool
	Arithmetic                uint32
	UseDatabase               bool
	ArithmeticWeight          uint32
	NonNestedArithmeticWeight uint32
	PullQuantifiers           bool
	NoPatternWeight           int32
	AvoidSkolems              bool
	Warnings                  bool
	WarningService            *diagnostic.WarningService
}

func DefaultInferenceOptions() InferenceOptions {
	return InferenceOptions{
		BlockLoopPatterns: true, Arithmetic: 1,
		ArithmeticWeight: 5, NonNestedArithmeticWeight: 10,
		PullQuantifiers: true, NoPatternWeight: -1, AvoidSkolems: true,
	}
}

func (o InferenceOptions) internal() quantifierInferenceOptions {
	return quantifierInferenceOptions{
		arithmeticMode: arithmeticPatternMode(o.Arithmetic),
		maxExtraMulti:  o.MaxMultiPatterns, noPatternWeight: o.NoPatternWeight,
		arithmeticWeight: o.ArithmeticWeight, nonNestedArithmeticWeight: o.NonNestedArithmeticWeight,
		useDatabase: o.UseDatabase, pullQuantifiers: o.PullQuantifiers,
		warnings: o.Warnings, warningService: o.WarningService,
	}
}
