package duality

import "errors"

// Derive initializes a fresh slow search over the caller-owned working graph.
// Graph/solver services are supplied explicitly; successful SAT results keep
// their counterexample graph/model after the temporary query scope is popped.
// Source: duality_solver.cpp:1879-1907, active non-caching path.
func (s *DerivationSearch) Derive(root *Node, underapprox, constrained bool) (sat bool, result error) {
	if s == nil || s.Search == nil || s.Search.DerivationTree == nil {
		return false, ErrWrapperOwner
	}
	d := s.Search
	if root == nil || root.closed || root.Owner != d.original || d.Top != nil || len(d.stack) != 1 || len(d.leaves) != 0 {
		return false, ErrWrapperOwner
	}
	if err := d.tree.solverReady(); err != nil {
		return false, err
	}
	timer, err := startProfile(d.tree.context, "Derive")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	// The source case loop operates on this derivation's RPFP. Share its
	// failure schedule while borrowing the current graph only for this query.
	if s.Cases != nil {
		previous := s.Cases.Graph
		s.Cases.Graph = d.tree
		defer func() { s.Cases.Graph = previous }()
	}
	d.Underapprox = underapprox
	d.Constrained = constrained
	d.FalseApprox = true
	d.tree.HornClauses = d.original.HornClauses
	baseScopes := len(d.tree.scopes)
	if err := d.tree.Push(); err != nil {
		return false, err
	}
	defer func() {
		popTimer, err := startProfile(d.tree.context, "Pop")
		if err != nil {
			result = errors.Join(result, err)
		}
		defer func() { result = errors.Join(result, popTimer.Close()) }()
		// An error may leave an incomplete expansion with no outgoing edge;
		// unwind native graph scopes directly rather than deleting that partial
		// graph through PopLevel. The caller still owns and closes the graph.
		for len(d.tree.scopes) > baseScopes {
			if err := d.tree.Pop(1); err != nil {
				result = errors.Join(result, err)
				return
			}
		}
		if result != nil {
			d.updatedNodes = nil
			d.stack = d.stack[:1]
			d.nodeMap = make(map[*Node][]*Node)
		}
	}()

	top, err := d.CreateApproximatedInstance(root)
	if err != nil {
		return false, err
	}
	d.Top = top
	if err = d.tree.AssertNode(top); err != nil {
		return false, err
	}
	buildTimer, err := startProfile(d.tree.context, "Build")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, buildTimer.Close()) }()
	if constrained {
		sat, err = s.buildFast()
	} else {
		sat, err = s.Build()
	}
	if err != nil {
		return sat, err
	}
	if err = d.Heuristic.Done(); err != nil {
		return false, err
	}
	if err := buildTimer.Close(); err != nil {
		return false, err
	}
	return sat, nil
}
