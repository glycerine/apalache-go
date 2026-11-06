package duality

// Source: duality_solver.cpp, DerivationTree::Build and CheckWithUnderapprox.
import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// buildFast drives the base derivation used for counterexample completion.
func (s *DerivationSearch) buildFast() (sat bool, result error) {
	if s == nil || s.Search == nil || s.Search.DerivationTree == nil || s.Simplifier == nil {
		return false, ErrWrapperOwner
	}
	d := s.Search.DerivationTree
	limit := int(^uint32(0) >> 1)
	for {
		if err := d.tree.checkpoint(); err != nil {
			return false, err
		}
		expanded, err := d.ExpandSomeNodes(true, limit)
		if err != nil {
			return false, err
		}
		if !expanded {
			break
		}
	}
	for {
		if err := d.tree.checkpoint(); err != nil {
			return false, err
		}
		checkUnder := false
		if (d.Underapprox || d.FalseApprox) && d.Top.Outgoing != nil {
			timer, err := startProfile(d.tree.context, "CheckWithUnderapprox")
			if err != nil {
				return false, err
			}
			status, err := d.tree.Check(d.Top, d.leaves, nil)
			err = errors.Join(err, timer.Close())
			if err != nil {
				return false, err
			}
			checkUnder = status != lbool.False
		}
		if !checkUnder {
			// Discard a proof derived with temporary leaf assumptions before
			// interpolating the unassumed constraint tree.
			if _, err := d.tree.solver.CheckSat(nil); err != nil {
				return false, err
			}
			var theory []*Expr
			if s.Cases != nil {
				theory = s.Cases.Theory
			}
			status, err := d.tree.solveWithTheory(d.Top, s.Simplifier, theory, s.Config)
			if err != nil {
				return false, err
			}
			if status == lbool.False {
				return false, nil
			}
		}
		if !checkUnder || d.Constrained {
			expanded, err := d.ExpandSomeNodes(false, limit)
			if err != nil {
				return false, err
			}
			if expanded {
				continue
			}
		}
		// The compiled source skips model-derived lower facts for constrained
		// completion (UNDERAPPROX_NODES is paired with
		// SKIP_UNDERAPPROX_NODES). Preserve the lower facts cloned into the
		// partial counterexample instead of publishing completion-model states.
		if d.Underapprox && !d.Constrained {
			timer, err := startProfile(d.tree.context, "ComputeUnderapprox")
			if err != nil {
				return false, err
			}
			fact, err := d.tree.ComputeUnderapprox(d.Top, s.Simplifier)
			if fact != nil {
				err = errors.Join(err, fact.Close())
			}
			err = errors.Join(err, timer.Close())
			if err != nil {
				return false, err
			}
		}
		return true, nil
	}
}
