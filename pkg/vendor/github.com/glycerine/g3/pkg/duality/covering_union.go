package duality

// Source: duality_solver.cpp:2763-2799 (active UNDERAPPROX_NODES).
import "errors"

// CoverByAll forms the source union of eligible earlier instance annotations.
// A successful collective cover uses the covered node itself as the nonnull
// marker; invalidation follows the same ordering rule as single-node covers.
func (c *Covering) CoverByAll(covered *Node) (_ bool, result error) {
	if c == nil || covered == nil || covered.closed || covered.Owner != c.graph {
		return false, ErrWrapperOwner
	}
	union, err := covered.Annotation.Clone()
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, union.Close()) }()
	if err = union.SetEmpty(); err != nil {
		return false, err
	}
	var others []*Node
	count := 0
	for _, candidate := range c.instances[covered.Map] {
		if !c.CoverOrder(candidate, covered) {
			continue
		}
		blocked, err := c.IsCovered(candidate)
		if err != nil {
			return false, err
		}
		if blocked {
			continue
		}
		count++
		others = append(others, candidate)
		if err = union.UnionWith(candidate.Annotation); err != nil {
			return false, err
		}
	}
	if count == 0 {
		return false, nil
	}
	subset, err := covered.Annotation.SubsetEq(union, c.aux)
	if err != nil || !subset {
		return false, err
	}
	c.coveredBy[covered] = covered
	if err := c.graph.streamReporter.AddCover(covered, others); err != nil {
		return false, err
	}
	if err := c.RemoveAscendantCoverings(covered); err != nil {
		return false, err
	}
	return true, nil
}
func (c *Covering) Close(node *Node) (bool, error) {
	if c == nil || node == nil || node.closed || node.Owner != c.graph {
		return false, ErrWrapperOwner
	}
	if c.coveredBy[node] != nil {
		return true, nil
	}
	return c.CoverByAll(node)
}
