package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
)

// ExpandNodeFromCoverFail replaces a nonleaf node with a covering
// underapproximation leaf, redirects a similar instance's incoming candidates
// depth-first, then activates the leaf through native induction expansion.
// The counterexample stays borrowed; the unwinding owns every created node.
// Source: duality_solver.cpp:749-767, active UNDERAPPROX_NODES branch.
func (u *Unwinding) ExpandNodeFromCoverFail(node *Node, counterexample *Counterexample, native solver.Solver, evaluator model.ModelEvaluatorConfig, simplifier *interp.Manager) (bool, error) {
	if u == nil || node == nil || node.closed || node.Owner != u.graph {
		return false, ErrWrapperOwner
	}
	if node.Outgoing == nil || len(node.Outgoing.Children) == 0 {
		return false, nil
	}
	other, err := u.covering.GetSimilarNode(node)
	if err != nil || other == nil {
		return false, err
	}
	under, err := u.CreateUnderapproxNode(node, counterexample)
	if err != nil {
		return false, err
	}
	if err = u.covering.MapUnderapproximation(under, node); err != nil {
		return false, err
	}
	if _, err = u.covering.CoverByNode(node, under); err != nil {
		return false, err
	}
	if err = u.ExpandNodeFromOther(under, other); err != nil {
		return false, err
	}
	if err = u.ExpandNode(under, native, evaluator, simplifier); err != nil {
		return false, err
	}
	return true, nil
}
