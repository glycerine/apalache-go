package duality

import "github.com/glycerine/g3/pkg/solver"

// Update intersects a new upper fact after invalidating covering relations.
// Learned facts are reported before intersecting the current annotation.
// Source: duality_solver.cpp:1561-1575.
func (u *Unwinding) Update(node *Node, fact *Transformer, aux solver.Solver) (bool, error) {
	return u.update(node, fact, aux, false)
}

func (u *Unwinding) update(node *Node, fact *Transformer, aux solver.Solver, eager bool) (bool, error) {
	if u == nil || node == nil || node.closed || node.Owner != u.graph || node.Map == nil || aux == nil {
		return false, ErrWrapperOwner
	}
	redundant, err := node.Annotation.SubsetEq(fact, aux)
	if err != nil {
		return false, err
	}
	if redundant {
		return false, nil
	}
	if err := u.graph.streamReporter.Update(node, fact, eager); err != nil {
		return false, err
	}
	if err := u.graph.conjectureReporter.Update(node, fact); err != nil {
		return false, err
	}
	if err := u.covering.Update(node); err != nil {
		return false, err
	}
	if u.updatedNodes == nil {
		u.updatedNodes = make(map[*Node]bool)
	}
	u.updatedNodes[node.Map] = true
	if err = node.Annotation.IntersectWith(fact); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateWithInterpolant traverses child positions before their parent, without
// deduplicating shared unwinding nodes. Source increments heuristic scores even
// when an upper fact is redundant. The unused C++ tree parameter is omitted.
// Source: duality_solver.cpp:1578-1585.
func (u *Unwinding) UpdateWithInterpolant(node, top *Node, aux solver.Solver, heuristic *ExpansionHeuristic) error {
	if err := u.updatePair(node, top, aux, heuristic); err != nil {
		return err
	}
	if top.Outgoing != nil {
		if node.Outgoing == nil || len(node.Outgoing.Children) != len(top.Outgoing.Children) {
			return ErrWrapperOwner
		}
		for i, child := range top.Outgoing.Children {
			if err := u.UpdateWithInterpolant(node.Outgoing.Children[i], child, aux, heuristic); err != nil {
				return err
			}
		}
	}
	if _, err := u.Update(node, top.Annotation, aux); err != nil {
		return err
	}
	return heuristic.Update(node)
}

// UpdateWithCounterexample unions genuinely new lower facts. It does not
// invalidate upper-fact covering relations or mark upper annotations updated.
// Source: duality_solver.cpp:1589-1601.
func (u *Unwinding) UpdateWithCounterexample(node, top *Node, aux solver.Solver, heuristic *ExpansionHeuristic) error {
	if err := u.updatePair(node, top, aux, heuristic); err != nil {
		return err
	}
	if top.Outgoing != nil {
		if node.Outgoing == nil || len(node.Outgoing.Children) != len(top.Outgoing.Children) {
			return ErrWrapperOwner
		}
		for i, child := range top.Outgoing.Children {
			if err := u.UpdateWithCounterexample(node.Outgoing.Children[i], child, aux, heuristic); err != nil {
				return err
			}
		}
	}
	redundant, err := top.Underapprox.SubsetEq(node.Underapprox, aux)
	if err != nil {
		return err
	}
	if redundant {
		return nil
	}
	if err := u.graph.streamReporter.UpdateUnderapprox(node, top.Underapprox); err != nil {
		return err
	}
	if err = node.Underapprox.UnionWith(top.Underapprox); err != nil {
		return err
	}
	return heuristic.Update(node)
}
func (u *Unwinding) updatePair(node, top *Node, aux solver.Solver, heuristic *ExpansionHeuristic) error {
	if u == nil || node == nil || top == nil || node.closed || top.closed || node.Owner != u.graph || top.Map != node || top.Owner.context != u.graph.context || aux == nil || heuristic == nil {
		return ErrWrapperOwner
	}
	return u.graph.open()
}
