package duality

import (
	"errors"
	"fmt"
	"os"
)

// ProveConjecture strengthens a node's bound and keeps it if proved. On an
// ordinary counterexample it optionally expands an expensive underapproximate
// failure, transfers the counterexample to output (if supplied), clears the
// current counterexample and restores the saved bound. Errors preserve the
// source partially strengthened state for the unwinding owner to inspect.
// Source: duality_solver.cpp:1154-1183, LOCALIZE_CONJECTURES disabled.
func (u *Unwinding) ProveConjecture(node *Node, conjecture *Transformer, search *DerivationSearch, counterexample, output *Counterexample, useUnderapprox bool) (proved bool, result error) {
	if u == nil || node == nil || node.closed || node.Owner != u.graph || counterexample == nil {
		return false, ErrWrapperOwner
	}
	if err := counterexample.open(); err != nil {
		return false, err
	}
	if output != nil {
		if err := output.open(); err != nil {
			return false, err
		}
	}
	if err := u.graph.streamReporter.Conjecture(node, conjecture); err != nil {
		return false, err
	}
	timer, err := startProfile(u.graph.context, "ProveConjecture")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	saved, err := node.Bound.Clone()
	if err != nil {
		return false, err
	}
	restored := false
	defer func() {
		if !restored {
			result = errors.Join(result, saved.Close())
		}
	}()
	if err = node.Bound.IntersectWith(conjecture); err != nil {
		return false, err
	}
	proved, err = u.SatisfyUpperBound(node, search, counterexample, useUnderapprox)
	if err != nil || proved {
		return proved, err
	}
	if useUnderapprox && u.lastDecisions > 500 {
		_, _ = fmt.Fprintln(os.Stdout, "making an underapprox")
		working := search.Search.tree
		if _, err = u.ExpandNodeFromCoverFail(node, counterexample, working.solver, working.evaluatorConfig, search.Simplifier); err != nil {
			return false, err
		}
	}
	if output != nil {
		if err = output.Swap(counterexample); err != nil {
			return false, err
		}
	}
	if err = counterexample.Clear(); err != nil {
		return false, err
	}
	if err = node.Bound.Close(); err != nil {
		return false, err
	}
	node.Bound = saved
	restored = true
	return false, nil
}
