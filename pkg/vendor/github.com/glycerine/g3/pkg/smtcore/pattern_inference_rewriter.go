package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/pattern"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// InferenceOptions snapshots the current pi module values for the AST rewriter.
func (s *SMTPatternInferenceParams) InferenceOptions(w *diagnostic.WarningService) (pattern.InferenceOptions, error) {
	if s == nil || !s.constructorReady {
		return pattern.InferenceOptions{}, uparams.ErrParamsPrecondition
	}
	state, err := s.State()
	if err != nil {
		return pattern.InferenceOptions{}, err
	}
	return pattern.InferenceOptions{
		MaxMultiPatterns:          state.MaxMultiPatterns,
		BlockLoopPatterns:         state.BlockLoopPatterns,
		Arithmetic:                state.Arithmetic,
		UseDatabase:               state.UseDatabase,
		ArithmeticWeight:          state.ArithmeticWeight,
		NonNestedArithmeticWeight: state.NonNestedArithmeticWeight,
		PullQuantifiers:           state.PullQuantifiers,
		NoPatternWeight:           state.NoPatternWeight,
		AvoidSkolems:              state.AvoidSkolems,
		Warnings:                  state.Warnings,
		WarningService:            w,
	}, nil
}

func (s *SMTPatternInferenceParams) NewInferenceRewriter(m *ast.ASTManager, w *diagnostic.WarningService) (*pattern.InferenceRewriter, error) {
	options, err := s.InferenceOptions(w)
	if err != nil {
		return nil, err
	}
	return pattern.NewInferenceRewriterWithOptions(m, options)
}

func (s *SMTPatternInferenceParams) UpdateInferenceRewriter(r *pattern.InferenceRewriter, w *diagnostic.WarningService) error {
	options, err := s.InferenceOptions(w)
	if err != nil {
		return err
	}
	return r.UpdateOptions(options)
}
