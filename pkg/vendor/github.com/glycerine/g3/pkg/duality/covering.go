package duality

// Source: duality_solver.cpp:2680-2760,2915-2918 (active UNDERAPPROX_NODES).
import "github.com/glycerine/g3/pkg/solver"

// Covering tracks the concrete single-node covering relation. The outer
// search registers new instances and underapproximation-node associations.
// Stream events borrow the owning search reporter through the graph.
type Covering struct {
	dominance   map[*Node]map[*Node]bool
	simpleLoops map[*Node]bool
	dominated   map[*Node]bool
	graph       *RPFP
	aux         solver.Solver
	coveredBy   map[*Node]*Node
	covers      map[*Node][]*Node
	allOfNode   map[*Node][]*Node
	instances   map[*Node][]*Node
	underapprox map[*Node]*Node
	someUpdates bool
}

func NewCovering(graph *RPFP, aux solver.Solver) (*Covering, error) {
	if err := graph.open(); err != nil {
		return nil, err
	}
	if aux == nil {
		return nil, ErrWrapperOwner
	}
	m, err := aux.Manager()
	if err != nil {
		return nil, err
	}
	if m != graph.context.manager {
		return nil, ErrContextMismatch
	}
	c := &Covering{dominance: make(map[*Node]map[*Node]bool), simpleLoops: make(map[*Node]bool), dominated: make(map[*Node]bool), instances: make(map[*Node][]*Node), graph: graph, aux: aux, coveredBy: make(map[*Node]*Node), covers: make(map[*Node][]*Node), allOfNode: make(map[*Node][]*Node), underapprox: make(map[*Node]*Node)}
	for _, node := range graph.nodes {
		if err = c.RegisterInstance(node); err != nil {
			return nil, err
		}
	}
	return c, nil
}
func (c *Covering) RegisterInstance(node *Node) error {
	if c == nil || node == nil || node.closed || node.Owner != c.graph {
		return ErrWrapperOwner
	}
	group := node.Map
	if group != nil {
		count := 0
		loop := false
		for _, edge := range group.Owner.edges {
			if edge.Parent == group {
				count++
				if len(edge.Children) == 1 && edge.Children[0] == group {
					loop = true
				}
			}
		}
		if count == 2 && loop {
			c.simpleLoops[group] = true
		}
	}
	for _, other := range c.allOfNode[group] {
		if other == node {
			return nil
		}
	}
	c.allOfNode[group] = append(c.allOfNode[group], node)
	c.instances[group] = append(c.instances[group], node)
	return nil
}
func (c *Covering) MapUnderapproximation(node, associated *Node) error {
	if c == nil || node == nil || node.closed || node.Owner != c.graph {
		return ErrWrapperOwner
	}
	c.underapprox[node] = associated
	return nil
}
func (c *Covering) CoverOrder(covering, covered *Node) bool {
	if _, found := c.underapprox[covered]; found {
		return false
	}
	if associated, found := c.underapprox[covering]; found {
		return covering.Number < covered.Number || associated == covered
	}
	return covering.Number < covered.Number
}
func (c *Covering) IsCovered(node *Node) (bool, error) {
	if c == nil {
		return false, ErrWrapperOwner
	}
	seen := make(map[*Node]bool)
	var visit func(*Node) (bool, error)
	visit = func(n *Node) (bool, error) {
		if n == nil || n.closed || n.Owner != c.graph {
			return false, ErrWrapperOwner
		}
		if seen[n] {
			return false, nil
		}
		seen[n] = true
		if c.coveredBy[n] != nil {
			return true, nil
		}
		if n.Outgoing == nil {
			return false, ErrWrapperOwner
		}
		for _, child := range n.Outgoing.Children {
			covered, err := visit(child)
			if err != nil || covered {
				return covered, err
			}
		}
		return false, nil
	}
	return visit(node)
}
func (c *Covering) RemoveCoveringsBy(node *Node) error {
	if c == nil || node == nil || node.closed || node.Owner != c.graph {
		return ErrWrapperOwner
	}
	for _, other := range c.allOfNode[node.Map] {
		if c.coveredBy[other] != nil && c.CoverOrder(node, other) {
			delete(c.coveredBy, other)
			if err := c.graph.streamReporter.RemoveCover(other, node); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *Covering) RemoveAscendantCoverings(node *Node) error {
	seen := make(map[*Node]bool)
	var visit func(*Node) error
	visit = func(n *Node) error {
		if seen[n] {
			return nil
		}
		seen[n] = true
		if err := c.RemoveCoveringsBy(n); err != nil {
			return err
		}
		for _, edge := range n.Incoming {
			if edge.Parent != nil {
				if err := visit(edge.Parent); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(node)
}
func (c *Covering) CheckCover(covered, covering *Node) (bool, error) {
	if c == nil || covered == nil || covering == nil || covered.Owner != c.graph || covering.Owner != c.graph || covered.closed || covering.closed {
		return false, ErrWrapperOwner
	}
	if !c.CoverOrder(covering, covered) {
		return false, nil
	}
	subset, err := covered.Annotation.SubsetEq(covering.Annotation, c.aux)
	if err != nil || !subset {
		return false, err
	}
	blocked, err := c.IsCovered(covering)
	return !blocked, err
}
func (c *Covering) CoverByNode(covered, covering *Node) (bool, error) {
	valid, err := c.CheckCover(covered, covering)
	if err != nil || !valid {
		return false, err
	}
	c.coveredBy[covered] = covering
	c.covers[covering] = append(c.covers[covering], covered)
	if err := c.graph.streamReporter.AddCover(covered, []*Node{covering}); err != nil {
		return false, err
	}
	if err := c.RemoveAscendantCoverings(covered); err != nil {
		return false, err
	}
	return true, nil
}
func (c *Covering) Update(node *Node) error {
	if err := c.RemoveCoveringsBy(node); err != nil {
		return err
	}
	c.someUpdates = true
	return nil
}

// SetInstanceActive updates the source insts_of_node candidate list while
// preserving all_of_node history for annotation-update invalidation.
func (c *Covering) SetInstanceActive(node *Node, active bool) error {
	if c == nil || node == nil || node.closed || node.Owner != c.graph {
		return ErrWrapperOwner
	}
	if err := c.RegisterInstance(node); err != nil {
		return err
	}
	group := node.Map
	for n, other := range c.instances[group] {
		if other == node {
			if !active {
				c.instances[group] = append(c.instances[group][:n], c.instances[group][n+1:]...)
			}
			return nil
		}
	}
	if active {
		c.instances[group] = append(c.instances[group], node)
	}
	return nil
}
