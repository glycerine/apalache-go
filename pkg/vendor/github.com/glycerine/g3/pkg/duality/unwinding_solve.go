package duality

// TestRecursionBounded records whether any uncovered active instance exceeds
// an original node's recursion limit. Covered instances do not affect results;
// each active cutoff publishes the source relation/depth message.
// Source: duality_solver.cpp:815-857.
func (u *Unwinding) TestRecursionBounded() error {
	if u == nil {
		return ErrWrapperOwner
	}
	u.recursionBounded = false
	if u.RecursionBound == -1 {
		return nil
	}
	for _, original := range u.source.nodes {
		for _, instance := range u.covering.instances[original] {
			contains, err := u.covering.Contains(instance)
			if err != nil {
				return err
			}
			if contains {
				past, err := u.nodePastRecursionBound(instance, true)
				if err != nil {
					return err
				}
				if past {
					u.recursionBounded = true
				}
			}
		}
	}
	return nil
}
func (u *Unwinding) IsResultRecursionBounded() bool { return u != nil && u.recursionBounded }

// SolveMain drives the active source outer loop over an initialized unwinding.
// True publishes the current inductive source solution and recursion status;
// false leaves an owning counterexample. Initial setup and inference-service teardown
// are separate lifecycle steps, as in source PreSolve/PostSolve.
// Source: duality_solver.cpp:1088-1123, active BOUNDED/EARLY_EXPAND switches.
func (s *UnwindingSearch) SolveMain() (bool, error) {
	if s == nil || s.Unwinding == nil {
		return false, ErrWrapperOwner
	}
	u := s.Unwinding
	if err := u.source.checkpoint(); err != nil {
		return false, err
	}
	if u.StratifiedInlining {
		ok, err := s.DoStratifiedInlining()
		if err != nil || !ok {
			return ok, err
		}
	}
	if err := u.DoTopoSort(); err != nil {
		return false, err
	}
	for {
		if err := u.source.checkpoint(); err != nil {
			return false, err
		}
		if err := s.ProduceCandidatesForExtension(); err != nil {
			return false, err
		}
		candidate, ok := u.NextCandidate()
		if !ok {
			if err := u.GenSolutionFromIndSet(false, s.Simplifier); err != nil {
				return false, err
			}
			if err := u.TestRecursionBounded(); err != nil {
				return false, err
			}
			return true, nil
		}
		feasible, err := u.CandidateFeasible(candidate, s.Native, s.Simplifier)
		if err != nil {
			return false, err
		}
		if !feasible {
			continue
		}
		node, satisfied, err := s.Extend(candidate)
		if err != nil || !satisfied {
			return satisfied, err
		}
		if err = s.tryExpandNode(node); err != nil {
			return false, err
		}
	}
}
