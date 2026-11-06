package duality

import "errors"

// ExtensionSearch borrows the concrete inference services and fresh working
// graphs for an extension. Bounds is also the service template for recursive
// underapproximation expansion. Completion is used only for a SAT result when
// UseUnderapprox is enabled. Working graphs are consumed only by searches that
// actually run, according to SatisfyUpperBound/BuildFullCex ownership rules.
type ExtensionSearch struct {
	Bounds, Completion *DerivationSearch
	Counterexample     *Counterexample
	Proposers          []*HistoryProposer
	UseUnderapprox     bool
}

// Extend creates an unwinding instance and keeps it solved through eager
// deduction, lazy upper-bound solving and covering/counterexample publication.
// Outgoing source edges and topological order must already be resolved. On an
// error the returned node remains owned by the unwinding graph for inspection.
// Source: duality_solver.cpp:1718-1738; reporter/timer hooks remain separate.
func (u *Unwinding) Extend(candidate ExpansionCandidate, services *ExtensionSearch) (node *Node, satisfied bool, result error) {
	if u == nil || candidate.Edge == nil || candidate.Edge.closed || candidate.Edge.Owner != u.source || services == nil || services.Bounds == nil || services.Bounds.Search == nil || services.Bounds.Search.DerivationTree == nil || services.Bounds.Search.original != u.graph || services.Counterexample == nil || u.topological == nil {
		return nil, false, ErrWrapperOwner
	}
	if err := services.Counterexample.open(); err != nil {
		return nil, false, err
	}
	if services.Bounds.Checker == nil || services.Bounds.Auxiliary == nil {
		return nil, false, ErrWrapperOwner
	}
	for _, child := range candidate.Children {
		if child == nil || child.closed || child.Owner != u.graph {
			return nil, false, ErrWrapperOwner
		}
	}
	timer, err := startProfile(u.graph.context, "Extend")
	if err != nil {
		return nil, false, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	node, result = u.CreateNodeInstance(candidate.Edge.Parent, 0)
	if result != nil {
		return node, false, result
	}
	if _, result = u.CreateEdgeInstance(candidate.Edge, node, candidate.Children); result != nil {
		return node, false, result
	}
	if result = u.graph.streamReporter.Extend(node); result != nil {
		return node, false, result
	}
	if result = u.UpdateBackEdges(node, u.topological); result != nil {
		return node, false, result
	}
	if result = u.DoEagerDeduction(node, services.Proposers, services.Bounds.Checker, services.Bounds.Auxiliary); result != nil {
		return node, false, result
	}
	satisfied, result = u.SatisfyUpperBound(node, services.Bounds, services.Counterexample, services.UseUnderapprox)
	if result != nil {
		return node, satisfied, result
	}
	if satisfied {
		_, result = u.covering.CloseDescendants(node)
		return node, true, result
	}
	tree, root := services.Counterexample.Tree(), services.Counterexample.Root()
	if result = u.ExpandUnderapproxNodes(tree, root, services.Bounds, services.UseUnderapprox); result != nil {
		return node, false, result
	}
	if services.UseUnderapprox {
		result = u.BuildFullCex(node, services.Completion, services.Counterexample, true)
	}
	return node, false, result
}
