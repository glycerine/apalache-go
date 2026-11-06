package duality

import "errors"

// ErrFullCounterexample is the source internal error raised when completing a
// previously feasible partial counterexample instead refutes its root.
var ErrFullCounterexample = errors.New("duality: internal error in BuildFullCex")

// BuildFullCex completes an underapproximate counterexample in constrained mode.
// It consumes the fresh search's working graph after validating its ownership:
// SAT transfers the graph to counterexample; UNSAT/errors close only the new
// graph and leave the previous counterexample available to its owner.
// Source: duality_solver.cpp:1689-1697.
func (u *Unwinding) BuildFullCex(node *Node, search *DerivationSearch, counterexample *Counterexample, useUnderapprox bool) (result error) {
	if u == nil || node == nil || node.closed || node.Owner != u.graph || search == nil || search.Search == nil || search.Search.DerivationTree == nil || search.Search.original != u.graph || counterexample == nil {
		return ErrWrapperOwner
	}
	if err := counterexample.open(); err != nil {
		return err
	}
	working := search.Search.tree
	if working == nil || working == u.graph || working == counterexample.Tree() {
		return ErrWrapperOwner
	}
	transferred := false
	defer func() {
		if !transferred {
			result = errors.Join(result, working.Close())
		}
	}()
	sat, err := search.Derive(node, useUnderapprox, true)
	if err != nil {
		return err
	}
	if !sat {
		return ErrFullCounterexample
	}
	if err = counterexample.Set(working, search.Search.Top); err != nil {
		return err
	}
	transferred = true
	return nil
}
