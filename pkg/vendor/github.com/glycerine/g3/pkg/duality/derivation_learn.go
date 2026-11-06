package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/solver"
)

// Cleanup may join another cause to a recoverable reduction failure.
// Only a chain containing exclusively the reduction failure can be ignored.
func onlyGreedyReduceFailure(err error) bool {
	if err == ErrGreedyReduceFailed {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !onlyGreedyReduceFailure(cause) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return onlyGreedyReduceFailure(wrapped)
	}
	return false
}

func (d *BacktrackingDerivation) refinementFailure(err error) error {
	if !onlyGreedyReduceFailure(err) {
		return err
	}
	if report := d.original.streamReporter.Message("interpolant verification failed"); report != nil {
		return errors.Join(err, report)
	}
	return nil
}

func (d *BacktrackingDerivation) reduceAnnotation(node *Node, manager *interp.Manager) (result error) {
	if manager == nil {
		return ErrWrapperOwner
	}
	m, err := manager.ASTManager()
	if err != nil {
		return err
	}
	if m != d.tree.context.manager {
		return ErrContextMismatch
	}
	formula, err := node.Annotation.Formula()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	reduced, err := d.tree.context.RemoveRedundancy(formula)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, reduced.Close()) }()
	h, err := reduced.Raw()
	if err != nil {
		return err
	}
	ref, err := manager.Simplify(h)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	h, err = m.RefGet(ref)
	if err != nil {
		return err
	}
	return m.SetRef(node.Annotation.formula, h)
}

// LearnCurrentExpansions connects the source per-node UNSAT learning pass.
// Source restart/error policy and iteration counters are handled by its caller.
// Source: duality_solver.cpp:2259-2309.
func (d *BacktrackingDerivation) LearnCurrentExpansions(checker *LocalEdgeChecker, aux solver.Solver, manager *interp.Manager, cases *CaseInterpolator, config *interp.RunConfig) (updates int, result error) {
	if d == nil || d.DerivationTree == nil || len(d.stack) == 0 || checker == nil || checker.closed || aux == nil || manager == nil {
		return 0, ErrWrapperOwner
	}
	expansions := d.stack[len(d.stack)-1].expansions
	var theory []*Expr
	if cases != nil {
		theory = cases.Theory
	}
	for _, node := range expansions {
		if node == nil || node.closed || node.Owner != d.tree || node.Map == nil {
			return updates, ErrWrapperOwner
		}
		refine := func() error {
			if _, err := d.tree.solveSingleNodeWithTheory(d.Top, node, manager, theory, config); err != nil {
				return err
			}
			complicated := false
			if len(expansions) == 1 {
				var err error
				complicated, err = d.NodeTooComplicated(node, manager)
				if err != nil {
					return err
				}
			}
			if complicated {
				if err := d.SimplifyNode(node, cases, config); err != nil {
					return err
				}
			} else {
				// Source performs this pass even when NodeTooComplicated already
				// simplified a singleton expansion before returning false.
				if err := d.reduceAnnotation(node, manager); err != nil {
					return err
				}
			}
			return checker.Generalize(node)
		}
		if err := d.refinementFailure(refine()); err != nil {
			return updates, err
		}
		changed, err := d.RecordAnnotationUpdate(node, aux)
		if err != nil {
			return updates, err
		}
		if changed {
			updates++
		} else {
			if err = d.Heuristic.Update(node.Map); err != nil {
				return updates, err
			}
		}
	}
	return updates, nil
}
