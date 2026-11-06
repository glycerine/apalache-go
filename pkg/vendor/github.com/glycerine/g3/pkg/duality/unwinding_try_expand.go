package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
)

// ExpansionSearch borrows native induction services and a fresh conjecture
// search. The conjecture search consumes its working graph only when used;
// Native remains separately borrowed after that graph has been released.
type ExpansionSearch struct {
	Conjecture                    *DerivationSearch
	Counterexample                *Counterexample
	Native                        solver.Solver
	Evaluator                     model.ModelEvaluatorConfig
	Simplifier                    *interp.Manager
	UseUnderapprox, NoConjectures bool
}

// Conjecture unions eligible earlier annotations in reverse instance order
// and tries the collective bound once, as in the active UNDERAPPROX_NODES path.
// Source: duality_solver.cpp:2893-2913.
func (c *Covering) Conjecture(node *Node, u *Unwinding, services *ExpansionSearch) (proved bool, result error) {
	if c == nil || u == nil || u.covering != c || node == nil || node.closed || node.Owner != c.graph {
		return false, ErrWrapperOwner
	}
	bound, err := node.Annotation.Clone()
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, bound.Close()) }()
	if err = bound.SetEmpty(); err != nil {
		return false, err
	}
	someOther := false
	instances := c.instances[node.Map]
	for i := len(instances) - 1; i >= 0; i-- {
		other := instances[i]
		eligible, err := c.CouldCover(node, other)
		if err != nil {
			return false, err
		}
		if eligible {
			if err := u.graph.streamReporter.Forcing(node, other); err != nil {
				return false, err
			}
			if err = bound.UnionWith(other.Annotation); err != nil {
				return false, err
			}
			someOther = true
		}
	}
	if !someOther {
		return false, nil
	}
	if services == nil {
		return false, ErrWrapperOwner
	}
	proved, err = u.ProveConjecture(node, bound, services.Conjecture, services.Counterexample, nil, services.UseUnderapprox)
	if err != nil || !proved {
		return proved, err
	}
	_, err = c.CloseDescendants(node)
	return true, err
}

// TryExpandNode first attempts collective covering, then collective conjecture
// proving, then dominance registration and induction expansion. A proved but
// still uncovered instance is activated without generating candidates, matching
// the source temporary workaround. Underapprox-covered failures stay pending.
// Source: duality_solver.cpp:1188-1212, active UNDERAPPROX_NODES switches.
func (u *Unwinding) TryExpandNode(node *Node, services *ExpansionSearch) error {
	if u == nil || node == nil || node.closed || node.Owner != u.graph {
		return ErrWrapperOwner
	}
	closed, err := u.covering.Close(node)
	if err != nil || closed {
		return err
	}
	if services == nil {
		return ErrWrapperOwner
	}
	if !services.NoConjectures {
		proved, err := u.covering.Conjecture(node, u, services)
		if err != nil {
			return err
		}
		if proved {
			contains, err := u.covering.Contains(node)
			if err != nil {
				return err
			}
			if contains {
				return u.ActivateInstance(node)
			}
			return nil
		}
	}
	contains, err := u.covering.Contains(node)
	if err != nil || !contains {
		return err
	}
	if err = u.covering.Add(node); err != nil {
		return err
	}
	return u.ExpandNode(node, services.Native, services.Evaluator, services.Simplifier)
}
