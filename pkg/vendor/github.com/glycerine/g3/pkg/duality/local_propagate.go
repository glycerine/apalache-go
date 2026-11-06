package duality

// Source: duality_solver.cpp:2583-2595; duality_rpfp.cpp:3211.
import "errors"

func (c *LocalEdgeChecker) Propagate(node *Node) (_ bool, result error) {
	if c == nil || c.closed || node == nil || node.closed || node.Map == nil || node.Outgoing == nil || node.Outgoing.Map == nil {
		return false, ErrWrapperOwner
	}
	original := node.Map
	if original.Outgoing == nil {
		return false, ErrWrapperOwner
	}
	source := node.Outgoing.Map
	edge, err := c.clones.GetEdgeClone(source)
	if err != nil {
		return false, err
	}
	if edge == nil {
		return false, ErrWrapperOwner
	}
	if len(edge.Children) != len(original.Outgoing.Children) {
		return false, ErrWrapperOwner
	}
	if err = replaceNodeAnnotation(edge.Parent, original.Annotation); err != nil {
		return false, err
	}
	for n, child := range edge.Children {
		if err = replaceNodeAnnotation(child, original.Outgoing.Children[n].Annotation); err != nil {
			return false, err
		}
	}
	state, err := c.pool.SolverForEdge(source, false, false, nil)
	if err != nil {
		return false, err
	}
	scope, err := state.Scope()
	if err != nil {
		return false, err
	}
	previous := c.graph.solver
	c.graph.solver = scope.Solver
	if c.graph.scopes == nil {
		c.graph.scopes = []graphScope{{}}
	}
	defer func() { c.graph.solver = previous; result = errors.Join(result, scope.Close()) }()
	cached, err := NewCachedGraphAssertions(c.graph, scope)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, cached.Close()) }()
	changed, err := cached.PropagateCache(edge)
	if err != nil {
		return false, err
	}
	if changed {
		if err = replaceNodeAnnotation(node, edge.Parent.Annotation); err != nil {
			return false, err
		}
	}
	return changed, nil
}
