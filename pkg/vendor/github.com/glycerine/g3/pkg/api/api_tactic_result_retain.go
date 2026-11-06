package api

// Source: src/api/api_tactic.cpp Z3_apply_result_inc_ref. Go returns a distinct
// owning handle whose goals and converters share the original result state.

import "errors"

func (r *APITacticResult) Retain() (result *APITacticResult, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	result = &APITacticResult{owner: r.owner, manager: r.manager}
	result.self = result
	owned := result
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, owned.Close())
			result = nil
		}
	}()
	for _, goal := range r.goals {
		if err := goal.IncRef(); err != nil {
			return nil, err
		}
		result.goals = append(result.goals, goal)
	}
	if r.modelConverter != nil {
		if err := r.modelConverter.IncRef(); err != nil {
			return nil, err
		}
		result.modelConverter = r.modelConverter
	}
	if r.proofConverter != nil {
		if err := r.proofConverter.IncRef(); err != nil {
			return nil, err
		}
		result.proofConverter = r.proofConverter
	}
	if err := result.core.Set(r.manager, r.core.Get()); err != nil {
		return nil, err
	}
	return result, nil
}
