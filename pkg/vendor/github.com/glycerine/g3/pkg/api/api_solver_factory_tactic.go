package api

// Source: src/api/api_solver.cpp:106-115.

func (c *SMTContext) NewSolverFromTactic(value *APITactic) (*APISimpleSolver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if err := value.open(); err != nil {
		return nil, err
	}
	if value.owner != c.smtContextState {
		return nil, ErrSMTContextOwner
	}
	created, err := c.command.NewSolverFromTactic(value.tactic)
	if err != nil {
		return nil, err
	}
	return &APISimpleSolver{owner: c.smtContextState, solver: created}, nil
}
