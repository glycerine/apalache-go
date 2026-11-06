package duality

// RemoveDeadNodes keeps exactly the nodes reachable through clause children
// from a node with a syntactically non-full bound. All clauses of a live
// predicate are followed, including those hidden by Node.Outgoing.
// Source: duality_rpfp.cpp, MarkLiveNodes/RemoveDeadNodes, FromClauses consumer.
func (g *RPFP) RemoveDeadNodes() error {
	if err := g.open(); err != nil {
		return err
	}
	outgoing := make(map[*Node][]*Edge)
	for _, edge := range g.edges {
		outgoing[edge.Parent] = append(outgoing[edge.Parent], edge)
	}
	live := make(map[*Node]bool)
	var pending []*Node
	for _, node := range g.nodes {
		full, err := node.Bound.IsFull()
		if err != nil {
			return err
		}
		if !full {
			pending = append(pending, node)
		}
	}
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if live[node] {
			continue
		}
		live[node] = true
		for _, edge := range outgoing[node] {
			pending = append(pending, edge.Children...)
		}
	}
	// Use graph deletion to release owned references and unlink every repeated
	// incoming occurrence. Copies preserve source order while deleting in place.
	for _, edge := range append([]*Edge(nil), g.edges...) {
		if !live[edge.Parent] {
			if err := g.DeleteEdge(edge); err != nil {
				return err
			}
		}
	}
	for _, node := range append([]*Node(nil), g.nodes...) {
		if !live[node] {
			if err := g.DeleteNode(node); err != nil {
				return err
			}
		}
	}
	return nil
}
