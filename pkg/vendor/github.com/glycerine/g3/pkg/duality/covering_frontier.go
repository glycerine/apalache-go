package duality

import "errors"

// Source: duality_solver.cpp:2801-2839.
func (c *Covering) CloseDescendants(node *Node) (_ bool, result error) {
	if c == nil {
		return false, ErrWrapperOwner
	}
	timer, err := startProfile(c.graph.context, "CloseDescendants")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	seen := make(map[*Node]bool)
	var visit func(*Node) (bool, error)
	visit = func(n *Node) (bool, error) {
		if n == nil || n.closed || n.Owner != c.graph || n.Outgoing == nil {
			return false, ErrWrapperOwner
		}
		if seen[n] {
			return false, nil
		}
		for _, child := range n.Outgoing.Children {
			closed, err := visit(child)
			if err != nil || closed {
				return closed, err
			}
		}
		closed, err := c.Close(n)
		if err != nil || closed {
			return closed, err
		}
		seen[n] = true
		return false, nil
	}
	return visit(node)
}
func (c *Covering) Contains(node *Node) (_ bool, result error) {
	if c == nil {
		covered, err := c.IsCovered(node)
		return !covered, err
	}
	var context *Context
	if c.graph != nil {
		context = c.graph.context
	}
	timer, err := startProfile(context, "Contains")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	covered, err := c.IsCovered(node)
	return !covered, err
}
func (c *Covering) Candidate(node *Node) (_ bool, result error) {
	if c == nil {
		return false, ErrWrapperOwner
	}
	var context *Context
	if c.graph != nil {
		context = c.graph.context
	}
	timer, err := startProfile(context, "Candidate")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	covered, err := c.IsCovered(node)
	if err != nil {
		return false, err
	}
	return !covered && !c.dominated[node], nil
}
func (c *Covering) SetDominated(node *Node) error {
	if c == nil || node == nil || node.closed || node.Owner != c.graph {
		return ErrWrapperOwner
	}
	c.dominated[node] = true
	return nil
}
