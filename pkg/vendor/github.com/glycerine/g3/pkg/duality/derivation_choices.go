package duality

// Source: duality_solver.cpp:2051-2058,2121-2142 (full-choice path).

// ExpansionChoicesFull considers active leaves, except high-priority calls
// offer all leaves to the heuristic without reading the counterexample model.
func (d *DerivationTree) ExpansionChoicesFull(best *[]*Node, highPriority, bestOnly bool) error {
	if d == nil || d.tree == nil || d.Heuristic == nil {
		return ErrWrapperOwner
	}
	if err := d.tree.open(); err != nil {
		return err
	}
	var choices []*Node
	seen := make(map[*Node]bool)
	for _, leaf := range d.leaves {
		active := highPriority
		if !highPriority {
			empty, err := d.tree.Empty(leaf)
			if err != nil {
				return err
			}
			active = !empty
		}
		if active && !seen[leaf] {
			choices = append(choices, leaf)
			seen[leaf] = true
		}
	}
	return d.Heuristic.ChooseExpand(choices, best, highPriority, bestOnly)
}

// ExpandSomeFullNodes applies the source full-choice expansion path to
// materialized original edges. Underapproximation selection is provided by
// ExpansionChoices; stratified edge resolution remains unfinished.
func (d *DerivationTree) ExpandSomeFullNodes(highPriority bool, limit int) (bool, error) {
	var choices []*Node
	if err := d.ExpansionChoicesFull(&choices, highPriority, limit != int(^uint32(0)>>1)); err != nil {
		return false, err
	}
	return d.expandSelected(choices, limit)
}

func (d *DerivationTree) expandSelected(choices []*Node, limit int) (bool, error) {
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
				// Keep the unprocessed leaf suffix owned by the search after a Go error.
				d.leaves = append(d.leaves, previous[n:]...)
				return len(choices) > 0, err
			}
		} else {
			d.leaves = append(d.leaves, node)
		}
	}
	// Source reports whether choices existed, even when limit prevented expansion.
	return len(choices) > 0, nil
}
