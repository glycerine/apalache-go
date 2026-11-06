package duality

// Source: duality_solver.cpp:2568-2580; duality_rpfp.cpp:3167.
import "errors"

func (c *LocalEdgeChecker) Generalize(node *Node) (result error) {
	if c == nil || c.closed || node == nil || node.closed || node.Outgoing == nil {
		return ErrWrapperOwner
	}
	if node.Outgoing.Map == nil {
		return nil
	}
	if node.Map == nil || node.Map.Outgoing == nil {
		return ErrWrapperOwner
	}
	source := node.Outgoing.Map
	edge, err := c.clones.GetEdgeClone(source)
	if err != nil {
		return err
	}
	if edge == nil {
		return ErrWrapperOwner
	}
	if len(edge.Children) != len(node.Map.Outgoing.Children) {
		return ErrWrapperOwner
	}
	if err = replaceNodeAnnotation(edge.Parent, node.Annotation); err != nil {
		return err
	}
	for n, child := range edge.Children {
		if err = replaceNodeAnnotation(child, node.Map.Outgoing.Children[n].Annotation); err != nil {
			return err
		}
	}
	state, err := c.pool.SolverForEdge(source, false, false, nil)
	if err != nil {
		return err
	}
	scope, err := state.Scope()
	if err != nil {
		return err
	}
	previous := c.graph.solver
	c.graph.solver = scope.Solver
	if c.graph.scopes == nil {
		c.graph.scopes = []graphScope{{}}
	}
	defer func() { c.graph.solver = previous; result = errors.Join(result, scope.Close()) }()
	cached, err := NewCachedGraphAssertions(c.graph, scope)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, cached.Close()) }()
	if err = cached.GeneralizeCache(edge, c.axioms); err != nil {
		return err
	}
	return replaceNodeAnnotation(node, edge.Parent.Annotation)
}
