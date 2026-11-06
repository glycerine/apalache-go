package duality

// CreateInitialUnwinding follows the active NEW_STRATIFIED_INLINING setup:
// stratified mode defers graph construction to DoStratifiedInlining; ordinary
// mode creates false leaves, then seeds nullary candidates or instantiates all
// edges when feasibility filtering is disabled.
// Source: duality_solver.cpp:385-400.
func (s *UnwindingSearch) CreateInitialUnwinding() error {
	if s == nil || s.Unwinding == nil {
		return ErrWrapperOwner
	}
	u := s.Unwinding
	if u.StratifiedInlining {
		return nil
	}
	services := &ExpansionSearch{Counterexample: s.Counterexample, Native: s.Native, Evaluator: s.Evaluator, Simplifier: s.Simplifier, UseUnderapprox: s.UseUnderapprox, NoConjectures: s.NoConjectures}
	if err := u.CreateLeaves(services); err != nil {
		return err
	}
	if u.FeasibleEdges {
		return u.NullaryCandidates()
	}
	return s.InstantiateAllEdges()
}

// InstantiateAllEdges extends every source edge over the initial leaves and
// performs early expansion. Source temporarily appends the initial leaves to
// active-instance lists even if early leaf expansion already activated them,
// then removes the first entry of each list; preserve that ordering exactly.
// The source ignores extension result booleans here, but propagates failures.
// Source: duality_solver.cpp:591-617, active EARLY_EXPAND.
func (s *UnwindingSearch) InstantiateAllEdges() error {
	if s == nil || s.Unwinding == nil {
		return ErrWrapperOwner
	}
	u := s.Unwinding
	leaves := make(map[*Node]*Node)
	for _, leaf := range u.leaves {
		leaves[leaf.Map] = leaf
		u.covering.instances[leaf.Map] = append(u.covering.instances[leaf.Map], leaf)
	}
	u.unexpanded = nil
	// Before SolveMain's topological sort, the source map supplies zero ranks.
	if u.topological == nil {
		u.topological = make(map[*Node]int)
	}
	for _, edge := range u.source.edges {
		children := make([]*Node, len(edge.Children))
		for i, original := range edge.Children {
			child := leaves[original]
			if child == nil {
				return ErrWrapperOwner
			}
			children[i] = child
		}
		node, _, err := s.Extend(ExpansionCandidate{Edge: edge, Children: children})
		if err != nil {
			return err
		}
		if err = s.tryExpandNode(node); err != nil {
			return err
		}
	}
	for _, node := range u.unexpanded {
		if err := u.covering.Add(node); err != nil {
			return err
		}
	}
	for _, leaf := range u.leaves {
		instances := u.covering.instances[leaf.Map]
		if len(instances) == 0 {
			return ErrWrapperOwner
		}
		instances[0] = nil
		u.covering.instances[leaf.Map] = instances[1:]
	}
	return nil
}
