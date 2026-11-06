package smtcore

// Source: src/api/api_context.cpp context::context's context_params value copy.

import uparams "github.com/glycerine/g3/pkg/util/params"

// CopyValuesFrom copies the source configuration fields while keeping the
// destination's own parameter services and budget.
func (s *ContextParams) CopyValuesFrom(source *ContextParams) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := source.open(); err != nil {
		return err
	}
	if !s.ready || !source.ready {
		return uparams.ErrParamsPrecondition
	}
	s.state = source.state
	s.dotProof = source.dotProof
	s.traceFile = source.traceFile
	return nil
}
