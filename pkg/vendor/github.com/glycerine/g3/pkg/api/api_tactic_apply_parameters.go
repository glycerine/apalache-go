package api

// Source: src/api/api_tactic.cpp:400-459, Z3_tactic_apply_ex.

// ApplyWithParameters validates parameters against this concrete tactic,
// updates the tactic, and applies it to an independent copy of goal. The
// parameter update remains on the tactic, matching the source apply_ex path.
func (t *APITactic) ApplyWithParameters(goal *APIGoal, parameters *ParameterSet) (*APITacticResult, error) {
	if err := t.validateParameters(parameters); err != nil {
		return nil, err
	}
	if err := goal.open(); err != nil {
		return nil, err
	}
	if goal.owner != t.owner {
		return nil, ErrSMTContextOwner
	}
	return t.apply(goal, parameters.params, &parameters.value)
}
