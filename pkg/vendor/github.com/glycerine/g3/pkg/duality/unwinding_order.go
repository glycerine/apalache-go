package duality

// Source: duality_solver.cpp:907-927.

// DoTopoSort follows source node/child order, marks nodes before descending
// to break cycles, and assigns postorder ranks. It deliberately visits only
// Node.Outgoing and appends a nil edge for a node with no outgoing edge.
func (u *Unwinding) DoTopoSort() error {
	if u == nil || u.source == nil {
		return ErrWrapperOwner
	}
	if err := u.source.open(); err != nil {
		return err
	}
	u.topological = make(map[*Node]int)
	u.sortedEdges = nil
	counter := 0
	var visit func(*Node) error
	visit = func(node *Node) error {
		if node == nil || node.closed || node.Owner != u.source {
			return ErrWrapperOwner
		}
		if _, seen := u.topological[node]; seen {
			return nil
		}
		u.topological[node] = int(^uint32(0) >> 1)
		edge := node.Outgoing
		if edge != nil {
			for _, child := range edge.Children {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		u.topological[node] = counter
		counter++
		u.sortedEdges = append(u.sortedEdges, edge)
		return nil
	}
	for _, node := range u.source.nodes {
		if err := visit(node); err != nil {
			return err
		}
	}
	return nil
}
