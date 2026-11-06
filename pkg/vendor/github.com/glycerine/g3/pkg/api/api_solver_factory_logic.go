package api

// Source: src/api/api_solver.cpp:87-104.

func (c *SMTContext) NewSolverForLogic(logic string) (*APISimpleSolver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	value, err := c.command.NewStrategicSolver(logic, true)
	if err != nil {
		return nil, err
	}
	return &APISimpleSolver{owner: c.smtContextState, solver: value}, nil
}
