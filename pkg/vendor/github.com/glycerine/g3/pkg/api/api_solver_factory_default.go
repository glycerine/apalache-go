package api

// Source: src/api/api_solver.cpp:76-85.

func (c *SMTContext) NewSolver() (*APISimpleSolver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	value, err := c.command.NewStrategicSolver("", false)
	if err != nil {
		return nil, err
	}
	return &APISimpleSolver{owner: c.smtContextState, solver: value}, nil
}
