package duality

// Source: duality_solver.cpp:2839-2860,2935-2949 (active source switches).
import "errors"

// CouldCover excludes simple source loops, already covered candidates and
// semantically empty annotations before proposing a conjecture.
func (c *Covering) CouldCover(covered, covering *Node) (_ bool, result error) {
	if c == nil || covered == nil || covering == nil || covered.closed || covering.closed || covered.Owner != c.graph || covering.Owner != c.graph {
		return false, ErrWrapperOwner
	}
	if c.simpleLoops[covered.Map] || !c.CoverOrder(covering, covered) {
		return false, nil
	}
	blocked, err := c.IsCovered(covering)
	if err != nil || blocked {
		return false, err
	}
	empty, err := covering.Annotation.Clone()
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, empty.Close()) }()
	if err = empty.SetEmpty(); err != nil {
		return false, err
	}
	subset, err := covering.Annotation.SubsetEq(empty, c.aux)
	return !subset, err
}

// GetSimilarNode searches current instances in reverse registration order.
// Unlike CouldCover, the active source branch does not test annotation size.
func (c *Covering) GetSimilarNode(node *Node) (*Node, error) {
	if c == nil || node == nil || node.closed || node.Owner != c.graph {
		return nil, ErrWrapperOwner
	}
	if !c.someUpdates {
		return nil, nil
	}
	instances := c.instances[node.Map]
	for n := len(instances) - 1; n >= 0; n-- {
		other := instances[n]
		if !c.CoverOrder(other, node) {
			continue
		}
		blocked, err := c.IsCovered(other)
		if err != nil {
			return nil, err
		}
		if !blocked {
			return other, nil
		}
	}
	return nil, nil
}
