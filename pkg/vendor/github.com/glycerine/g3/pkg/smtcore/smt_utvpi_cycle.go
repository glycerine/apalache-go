package smtcore

// Source: src/smt/diff_logic.h traverse_neg_cycle2, instantiated by
// theory_utvpi<rdl_ext>.

func (g *smtUTVPIGraph) negativeCycleEdges() ([]int32, error) {
	return g.graph.negativeCycleEdgesWithShortcuts()
}
