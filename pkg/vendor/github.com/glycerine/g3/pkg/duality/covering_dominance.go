package duality

// Source: duality_solver.cpp:2949-2977.
import "github.com/glycerine/g3/pkg/ast"

// Dominates compares mapped expansion structure using previously recorded
// child dominance. A null edge map denotes an unexpanded approximation.
func (c *Covering) Dominates(node, other *Node) (bool, error) {
	if c == nil || node == nil || other == nil || node.closed || other.closed || node.Owner != c.graph || other.Owner != c.graph {
		return false, ErrWrapperOwner
	}
	if node == other {
		return false, nil
	}
	if other.Outgoing == nil {
		return false, ErrWrapperOwner
	}
	if other.Outgoing.Map == nil {
		return true, nil
	}
	if node.Outgoing == nil {
		return false, ErrWrapperOwner
	}
	if node.Outgoing.Map != other.Outgoing.Map {
		return false, nil
	}
	if len(node.Outgoing.Children) != len(other.Outgoing.Children) {
		return false, ast.ErrASTDomain
	}
	for n, child := range node.Outgoing.Children {
		previous := other.Outgoing.Children[n]
		if child == previous {
			continue
		}
		if previous.Outgoing == nil {
			return false, ErrWrapperOwner
		}
		if previous.Outgoing.Map == nil || c.dominance[child][previous] {
			continue
		}
		return false, nil
	}
	return true, nil
}

// Add records dominance over current instances of the same original node.
func (c *Covering) Add(node *Node) error {
	if c == nil || node == nil || node.closed || node.Owner != c.graph {
		return ErrWrapperOwner
	}
	for _, other := range c.instances[node.Map] {
		dominates, err := c.Dominates(node, other)
		if err != nil {
			return err
		}
		if dominates {
			if c.dominance[node] == nil {
				c.dominance[node] = make(map[*Node]bool)
			}
			c.dominance[node][other] = true
			c.dominated[other] = true
			if err := c.graph.streamReporter.Dominates(node, other); err != nil {
				return err
			}
		}
	}
	return nil
}
