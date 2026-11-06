package duality

import "errors"

// SatisfyUpperBound runs a fresh resolved-edge derivation and publishes its
// result. After the full-bound shortcut, this call consumes the working graph:
// a SAT result transfers it to counterexample, and UNSAT/error paths close it.
// Solver/factory services and the unwinding graph remain borrowed.
// Source: duality_solver.cpp:1604-1633.
func (u *Unwinding) SatisfyUpperBound(node *Node, search *DerivationSearch, counterexample *Counterexample, useUnderapprox bool) (satisfied bool, result error) {
	if u == nil || node == nil || node.closed || node.Owner != u.graph {
		return false, ErrWrapperOwner
	}
	full, err := node.Bound.IsFull()
	if err != nil {
		return false, err
	}
	if full {
		return true, nil
	}
	if search == nil || search.Search == nil || search.Search.DerivationTree == nil || search.Search.original != u.graph || counterexample == nil {
		return false, ErrWrapperOwner
	}
	if err = counterexample.open(); err != nil {
		return false, err
	}
	working := search.Search.tree
	if working == nil || working == u.graph || working == counterexample.Tree() {
		return false, ErrWrapperOwner
	}
	transferred := false
	defer func() {
		if !transferred {
			result = errors.Join(result, working.Close())
		}
	}()
	if err := u.graph.streamReporter.Bound(node); err != nil {
		return false, err
	}
	start, err := working.CumulativeDecisions()
	if err != nil {
		return false, err
	}
	sat, err := search.Derive(node, useUnderapprox, false)
	if err != nil {
		return false, err
	}
	end, err := working.CumulativeDecisions()
	if err != nil {
		return false, err
	}
	u.lastDecisions = end - start
	top := search.Search.Top
	if sat {
		if err = counterexample.Set(working, top); err != nil {
			return false, err
		}
		transferred = true
		if useUnderapprox {
			if err = u.UpdateWithCounterexample(node, top, search.Auxiliary, search.Search.Heuristic); err != nil {
				return false, err
			}
		}
	} else {
		if err = u.UpdateWithInterpolant(node, top, search.Auxiliary, search.Search.Heuristic); err != nil {
			return false, err
		}
	}
	return !sat, nil
}
