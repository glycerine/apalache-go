package duality

import "github.com/glycerine/g3/pkg/solver"

// BacktrackUnsatisfiable performs the source post-learning backtracking loop.
// The caller first learns and records the current expansion's annotations.
// True means the search reached its base level and refuted this derivation.
// Source: duality_solver.cpp:2330-2358.
func (d *BacktrackingDerivation) BacktrackUnsatisfiable(checker *LocalEdgeChecker, aux solver.Solver) (bool, error) {
	if d == nil || d.DerivationTree == nil || len(d.stack) < 2 || checker == nil || checker.closed || aux == nil {
		return false, ErrWrapperOwner
	}
	if err := d.tree.ComputeProofCore(); err != nil {
		return false, err
	}
	propagated := false
	for {
		used, err := d.LevelUsedInProof(len(d.stack) - 2)
		if err != nil {
			return false, err
		}
		if err = d.PopLevel(); err != nil {
			return false, err
		}
		if len(d.stack) == 1 {
			break
		}
		if used {
			node := d.stack[len(d.stack)-1].expansions[0]
			changed, err := checker.Propagate(node)
			if err != nil {
				return false, err
			}
			if !changed {
				break
			}
			changed, err = d.RecordAnnotationUpdate(node, aux)
			if err != nil {
				return false, err
			}
			if !changed {
				break
			}
			if err = d.RemoveUpdateNodesAtCurrentLevel(); err != nil {
				return false, err
			}
			propagated = true
			continue
		}
		// Propagation changes facts used by the proof; after that, do not skip
		// another irrelevant expansion using the old proof core.
		if propagated {
			break
		}
		if err = d.RemoveUpdateNodesAtCurrentLevel(); err != nil {
			return false, err
		}
		for _, node := range d.stack[len(d.stack)-1].expansions {
			if err = d.Heuristic.Update(node.Map); err != nil {
				return false, err
			}
		}
	}
	if err := d.HandleUpdatedNodes(); err != nil {
		return false, err
	}
	if len(d.stack) == 1 {
		if d.Top != nil && d.Top.Outgoing != nil {
			if err := d.tree.DeleteEdge(d.Top.Outgoing); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
}
