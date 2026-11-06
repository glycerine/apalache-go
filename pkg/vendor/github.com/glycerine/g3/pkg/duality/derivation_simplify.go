package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
)

// NodeTooComplicated preserves the source pre-simplification >10 shortcut.
// Otherwise it replaces the annotation with redundancy removal plus native
// simplification, and tests whether more than three operators remain.
// Source: duality_solver.cpp:2448-2453.
func (d *BacktrackingDerivation) NodeTooComplicated(node *Node, manager *interp.Manager) (out bool, result error) {
	if d == nil || d.DerivationTree == nil || node == nil || node.closed || node.Owner != d.tree {
		return false, ErrWrapperOwner
	}
	formula, err := node.Annotation.Formula()
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	count, err := CountOperators(formula)
	if err != nil {
		return false, err
	}
	if count > 10 {
		return true, nil
	}
	if manager == nil {
		return false, ErrWrapperOwner
	}
	native, err := manager.ASTManager()
	if err != nil {
		return false, err
	}
	if native != d.tree.context.manager {
		return false, ErrContextMismatch
	}
	reduced, err := d.tree.context.RemoveRedundancy(formula)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, reduced.Close()) }()
	h, err := reduced.Raw()
	if err != nil {
		return false, err
	}
	ref, err := manager.Simplify(h)
	if err != nil {
		return false, err
	}
	m := d.tree.context.manager
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	h, err = m.RefGet(ref)
	if err != nil {
		return false, err
	}
	if err = m.SetRef(node.Annotation.formula, h); err != nil {
		return false, err
	}
	simplified, err := node.Annotation.Formula()
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, simplified.Close()) }()
	count, err = CountOperators(simplified)
	return count > 3, err
}

// SimplifyNode destroys the old native proof while replaying the current
// graph scope, then dispatches case interpolation. Source Bad errors propagate.
// Source: duality_solver.cpp:2455-2467.
func (d *BacktrackingDerivation) SimplifyNode(node *Node, runner *CaseInterpolator, config *interp.RunConfig) (result error) {
	if d == nil || d.DerivationTree == nil || runner == nil || runner.Graph != d.tree || node == nil || node.Owner != d.tree || node.closed || d.Top == nil {
		return ErrWrapperOwner
	}
	timer, err := startProfile(d.tree.context, "SimplifyNode")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	if err := d.tree.PopPush(); err != nil {
		return err
	}
	return runner.Interpolate(d.Top, node, config)
}
