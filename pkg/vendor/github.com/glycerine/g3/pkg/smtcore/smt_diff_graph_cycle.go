package smtcore

// Source: src/smt/diff_logic.h traverse_neg_cycle2 and check_explanation.

// negativeCycleEdgesWithShortcuts preserves strict inconsistency while using
// enabled weaker edges to shorten the parent cycle. The native try_relax
// argument is unused. Theory UTVPI's new_edge is empty; difference logic's
// callback is disabled by constructor-fixed m_theory_resolve=false with no
// retained writer. Neither concrete consumer needs a generic cut callback.
func (graph *SMTInfRationalDiffGraph) negativeCycleEdgesWithShortcuts() ([]int32, error) {
	if !graph.validEdge(graph.lastEnabledEdge) || graph.edgeFeasible(&graph.edges[graph.lastEnabledEdge]) {
		return nil, errSMTDiffGraphDomain
	}
	last := graph.lastEnabledEdge
	var gamma, potential smtInfRational
	gamma.Set(&graph.gamma[graph.edges[last].source])
	if gamma.Sign() >= 0 {
		return nil, errSMTDiffGraphDomain
	}
	potentials := []smtInfRational{}
	nodes := []int32{}
	edges := []int32{}
	id := last
	for steps := 0; ; steps++ {
		if steps > len(graph.edges) || !graph.validEdge(id) {
			return nil, errSMTDiffGraphDomain
		}
		edges = append(edges, id)
		edge := &graph.edges[id]
		source := edge.source
		potential.Add(&potential, &edge.weight)
		for _, candidateID := range graph.outEdges[source] {
			candidate := &graph.edges[candidateID]
			if candidateID == id || !candidate.enabled {
				continue
			}
			for j, target := range nodes {
				if target != candidate.target {
					continue
				}
				var delta, relaxed smtInfRational
				delta.Sub(&candidate.weight, &potential)
				delta.Add(&delta, &potentials[j])
				relaxed.Add(&gamma, &delta)
				if delta.Sign() >= 0 && relaxed.Sign() < 0 {
					gamma.Set(&relaxed)
					nodes = nodes[:j+1]
					potentials = potentials[:j+1]
					edges = append(edges[:j+1], candidateID)
					potential.Add(&potentials[j], &candidate.weight)
					break
				}
			}
		}
		potentials = append(potentials, *new(smtInfRational).Set(&potential))
		nodes = append(nodes, source)
		id = graph.parent[source]
		if id == last {
			break
		}
	}
	var weight smtInfRational
	for i, id := range edges {
		previous := edges[(i+len(edges)-1)%len(edges)]
		edge := &graph.edges[id]
		if edge.target != graph.edges[previous].source {
			return nil, errSMTDiffGraphDomain
		}
		weight.Add(&weight, &edge.weight)
	}
	if weight.Sign() >= 0 {
		return nil, errSMTDiffGraphDomain
	}
	return edges, nil
}
