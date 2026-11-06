package duality

// Source: duality_solver.cpp:516-541,974-989.

// CreateLeaf builds the source do_not_expand leaf without invoking eager
// expansion. Its lower-bound edge and final annotation are both false.
func (u *Unwinding) CreateLeaf(source *Node) (*Node, error) {
	node, err := u.CreateNodeInstance(source, 0)
	if err != nil {
		return nil, err
	}
	if err = u.MakeLeaf(node, true, nil); err != nil {
		return nil, err
	}
	if err = node.Annotation.SetEmpty(); err != nil {
		return nil, err
	}
	return node, nil
}
func (u *Unwinding) MakeOverapprox(node *Node) error {
	if u == nil || node == nil || node.Owner != u.graph || node.closed {
		return ErrWrapperOwner
	}
	if err := node.Annotation.SetFull(); err != nil {
		return err
	}
	edge, err := u.graph.CreateLowerBoundEdge(node)
	if err != nil {
		return err
	}
	edge.Map = nil
	if u.overapproximations == nil {
		u.overapproximations = make(map[*Node]bool)
	}
	u.overapproximations[node] = true
	return nil
}

// MarkExpanded activates only nodes still in the source pending set. Repeated
// calls do not append duplicate active instances.
func (u *Unwinding) MarkExpanded(node *Node) error {
	if u == nil || node == nil || node.Owner != u.graph || node.closed {
		return ErrWrapperOwner
	}
	for _, pending := range u.unexpanded {
		if pending.Number == node.Number {
			return u.ActivateInstance(node)
		}
	}
	return nil
}

// MakeLeaf installs the source false lower rule and optionally runs early
// covering/conjecture/induction expansion. Stratified mode keeps the annotation
// full for covering, while the edge remains false.
// Source: duality_solver.cpp:517-535, TOP_DOWN disabled and EARLY_EXPAND enabled.
func (u *Unwinding) MakeLeaf(node *Node, doNotExpand bool, services *ExpansionSearch) error {
	if u == nil || node == nil || node.closed || node.Owner != u.graph {
		return ErrWrapperOwner
	}
	if err := node.Annotation.SetEmpty(); err != nil {
		return err
	}
	edge, err := u.graph.CreateLowerBoundEdge(node)
	if err != nil {
		return err
	}
	if u.StratifiedInlining {
		if err = node.Annotation.SetFull(); err != nil {
			return err
		}
	} else {
		if u.updatedNodes == nil {
			u.updatedNodes = make(map[*Node]bool)
		}
		u.updatedNodes[node] = true
	}
	edge.Map = nil
	if err := u.graph.streamReporter.Extend(node); err != nil {
		return err
	}
	if !doNotExpand {
		return u.TryExpandNode(node, services)
	}
	return nil
}

// CreateLeaves resets the pending/leaf lists and creates source-order initial
// instances. Ordinary leaves are false and enter early expansion; stratified
// leaves are pending full overapproximations registered in the shared leaf map.
// Source: duality_solver.cpp:544-561, active switches.
func (u *Unwinding) CreateLeaves(services *ExpansionSearch) error {
	if u == nil {
		return ErrWrapperOwner
	}
	if err := u.source.open(); err != nil {
		return err
	}
	u.unexpanded = nil
	u.leaves = nil
	for _, original := range u.source.nodes {
		node, err := u.CreateNodeInstance(original, 0)
		if err != nil {
			return err
		}
		if !u.StratifiedInlining {
			if err = u.MakeLeaf(node, false, services); err != nil {
				return err
			}
		} else {
			if err = u.MakeOverapprox(node); err != nil {
				return err
			}
			if err = u.RegisterStratifiedLeaf(original, node); err != nil {
				return err
			}
		}
		u.leaves = append(u.leaves, node)
	}
	return nil
}

// NullaryCandidates seeds the queue from source edges with no children, in
// source order. Initialization invokes this directly rather than the ordinary
// candidate filter, so stratified duplicate filtering does not apply.
// Source: duality_solver.cpp:580-590.
func (u *Unwinding) NullaryCandidates() error {
	if u == nil {
		return ErrWrapperOwner
	}
	if err := u.source.open(); err != nil {
		return err
	}
	for _, edge := range u.source.edges {
		if len(edge.Children) == 0 {
			u.candidates = append(u.candidates, ExpansionCandidate{Edge: edge})
		}
	}
	return nil
}
