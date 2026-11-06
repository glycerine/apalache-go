package duality

import "errors"

// ExpandSomeNodes uses the slow search's expansion override so every selected
// materialized node records its backtracking level and child-instance mappings.
// Source: duality_solver.cpp:2121-2142,2502-2512.
func (d *BacktrackingDerivation) ExpandSomeNodes(highPriority bool, limit int) (_ bool, result error) {
	if d == nil || d.DerivationTree == nil {
		return false, ErrWrapperOwner
	}
	timer, err := startProfile(d.tree.context, "ExpandSomeNodes")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	choicesTimer, err := startProfile(d.tree.context, "ExpansionChoices")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, choicesTimer.Close()) }()
	var choices []*Node
	if err := d.ExpansionChoices(&choices, highPriority, limit != int(^uint32(0)>>1)); err != nil {
		return false, err
	}
	if err := choicesTimer.Close(); err != nil {
		return false, err
	}
	selected := make(map[*Node]bool)
	for _, node := range choices {
		selected[node] = true
	}
	previous := d.leaves
	d.leaves = nil
	count := 0
	for n, node := range previous {
		if selected[node] && count < limit {
			count++
			if err := d.ExpandMaterializedNode(node); err != nil {
				d.leaves = append(d.leaves, previous[n:]...)
				return len(choices) > 0, err
			}
		} else {
			d.leaves = append(d.leaves, node)
		}
	}
	return len(choices) > 0, nil
}

// AdvanceSatisfiable performs BuildMain's SAT branch. It fixes the preceding
// expansion's model decisions in a new scope and expands one selected leaf.
// False means no choices remain: expansion scopes are unwound while the graph
// itself is preserved for counterexample extraction. The disabled C++ Reduce
// routine's unused node_order vector is not copied.
// Source: duality_solver.cpp:2359-2401.
func (d *BacktrackingDerivation) AdvanceSatisfiable() (expanded bool, result error) {
	if d == nil || d.DerivationTree == nil || len(d.stack) == 0 {
		return false, ErrWrapperOwner
	}
	if err := d.tree.solverReady(); err != nil {
		return false, err
	}
	if d.tree.dualModel == nil {
		return false, ErrWrapperOwner
	}
	depth := len(d.stack)
	decisionScope := true
	if err := d.tree.Push(); err != nil {
		return false, err
	}
	// If selection fails before creating an expansion level, its decision scope
	// can be removed completely. Partially created expansions stay caller-owned.
	defer func() {
		if result != nil && decisionScope && len(d.stack) == depth {
			result = errors.Join(result, d.tree.Pop(1))
		}
	}()
	for _, node := range d.stack[depth-1].expansions {
		if node == nil {
			return false, ErrWrapperOwner
		}
		if err := d.tree.FixCurrentState(node.Outgoing); err != nil {
			return false, err
		}
	}
	expanded, err := d.ExpandSomeNodes(false, 1)
	if err != nil {
		return expanded, err
	}
	if expanded {
		return true, nil
	}
	decisionScope = false
	if err = d.tree.Pop(1); err != nil {
		return false, err
	}
	// Source completion uses native graph Pop, not PopLevel: edges and child
	// instances are the discovered counterexample and must survive completion.
	for len(d.stack) > 1 {
		if err = d.tree.Pop(1); err != nil {
			return false, err
		}
		d.stack = d.stack[:len(d.stack)-1]
	}
	return false, nil
}
