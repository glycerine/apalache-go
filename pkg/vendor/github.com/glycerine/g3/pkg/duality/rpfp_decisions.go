package duality

// CumulativeDecisions reports the borrowed native solver's cumulative count.
// Source: duality_rpfp.cpp:111-122, active get_num_decisions path.
func (g *RPFP) CumulativeDecisions() (int, error) {
	if err := g.solverReady(); err != nil {
		return 0, err
	}
	return nativeSolverDecisions(g.solver)
}
