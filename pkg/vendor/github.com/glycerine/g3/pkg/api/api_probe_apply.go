package api

// Source: src/api/api_tactic.cpp:431-437.

func (p *APIProbe) Apply(goal *APIGoal) (float64, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	if err := goal.open(); err != nil {
		return 0, err
	}
	if p.owner != goal.owner {
		return 0, ErrSMTContextOwner
	}
	return p.probe.Eval(goal.goal)
}
