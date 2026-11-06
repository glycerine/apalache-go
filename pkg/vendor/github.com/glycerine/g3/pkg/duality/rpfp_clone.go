package duality

// Source: duality_rpfp.cpp:1221-1234,1242-1244, active RPFP_caching clone path.

// RPFPCloneMap is the source edge-to-local-checker map. Each edge has its own
// parent and child instances; node copies are deliberately not shared across
// edges or even repeated child positions.
type RPFPCloneMap struct {
	graph *RPFP
	edges map[*Edge]*Edge
}

func NewRPFPCloneMap(graph *RPFP) (*RPFPCloneMap, error) {
	if err := graph.open(); err != nil {
		return nil, err
	}
	return &RPFPCloneMap{graph: graph, edges: make(map[*Edge]*Edge)}, nil
}
func (c *RPFPCloneMap) Clone(source *RPFP) error {
	if c == nil || c.graph == nil {
		return ErrWrapperOwner
	}
	if err := c.graph.open(); err != nil {
		return err
	}
	if err := source.open(); err != nil {
		return err
	}
	if c.graph == source {
		return ErrWrapperOwner
	}
	if c.graph.context != source.context {
		return ErrContextMismatch
	}
	for _, edge := range source.edges {
		parent, err := c.graph.CloneNode(edge.Parent)
		if err != nil {
			return err
		}
		children := make([]*Node, len(edge.Children))
		for n, child := range edge.Children {
			children[n], err = c.graph.CloneNode(child)
			if err != nil {
				return err
			}
		}
		copy, err := c.graph.CreateEdge(parent, edge.F, children)
		if err != nil {
			return err
		}
		c.edges[edge] = copy
	}
	return nil
}
func (c *RPFPCloneMap) GetEdgeClone(source *Edge) (*Edge, error) {
	if c == nil || c.graph == nil {
		return nil, ErrWrapperOwner
	}
	if err := c.graph.open(); err != nil {
		return nil, err
	}
	return c.edges[source], nil
}
