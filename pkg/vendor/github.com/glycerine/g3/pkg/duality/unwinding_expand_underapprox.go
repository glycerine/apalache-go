package duality

import "errors"

var ErrBogusUnderapproximation = errors.New("duality: bogus underapproximation")

// ExpandUnderapproxNodes rederives model states hidden by underapproximation
// leaves in the same borrowed counterexample graph. Services supplies concrete
// inference services and search policy; each recursive derivation gets fresh
// bookkeeping and shares its heuristic. Source leaves each model-state bound
// installed (its saved bound is not restored) and appends derivation roots
// without replacing the caller's root or splicing its edges.
// Source: duality_solver.cpp:1740-1762.
func (u *Unwinding) ExpandUnderapproxNodes(tree *RPFP, root *Node, services *DerivationSearch, useUnderapprox bool) error {
	if u == nil || tree == nil || root == nil || root.closed || root.Owner != tree || tree == u.graph || tree.context != u.graph.context {
		return ErrWrapperOwner
	}
	if err := tree.open(); err != nil {
		return err
	}
	original, found := u.covering.underapprox[root.Map]
	if found {
		if original == nil || original.closed || original.Owner != u.graph || services == nil || services.Search == nil || services.Search.DerivationTree == nil || services.Search.original != u.graph {
			return ErrWrapperOwner
		}
		constraint, err := tree.EvalNodeAsConstraint(root)
		if err != nil {
			return err
		}
		if err = constraint.Complement(); err != nil {
			return errors.Join(err, constraint.Close())
		}
		if err = original.Bound.Close(); err != nil {
			return errors.Join(err, constraint.Close())
		}
		original.Bound = constraint
		base, err := NewDerivationTree(u.graph, tree)
		if err != nil {
			return err
		}
		base.FullExpand = services.Search.FullExpand
		base.Heuristic = services.Search.Heuristic
		derivation, err := NewBacktrackingDerivation(base)
		if err != nil {
			return err
		}
		derivation.sourceCovering = u.covering
		search := &DerivationSearch{Search: derivation, Checker: services.Checker, Auxiliary: services.Auxiliary, Simplifier: services.Simplifier, Cases: services.Cases, Config: services.Config, EnableRestarts: services.EnableRestarts}
		sat, err := search.Derive(original, useUnderapprox, true)
		if err != nil {
			return err
		}
		if !sat {
			if err = u.UpdateWithInterpolant(original, base.Top, search.Auxiliary, base.Heuristic); err != nil {
				return err
			}
			return ErrBogusUnderapproximation
		}
		return u.ExpandUnderapproxNodes(tree, base.Top, services, useUnderapprox)
	}
	if root.Outgoing != nil {
		for _, child := range root.Outgoing.Children {
			if err := u.ExpandUnderapproxNodes(tree, child, services, useUnderapprox); err != nil {
				return err
			}
		}
	}
	return nil
}
