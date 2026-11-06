package duality

// Source: duality_solver.cpp:2060-2117.
import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/lbool"
	"io"
)

var ErrExpansionChoices = errors.New("error in ExpansionChoices")

func (d *DerivationTree) expansionChoicesRec(unused *[]*Node, used []*Node, choices map[*Node]bool, from, to int) error {
	if from == to {
		return nil
	}
	original := len(*unused)
	*unused = append(*unused, used[from:to]...)
	rejected := d.Top.Outgoing == nil
	if !rejected {
		status, err := d.tree.Check(d.Top, *unused, nil)
		if err != nil {
			return err
		}
		rejected = status == lbool.False
	}
	output := d.Output
	if output == nil {
		output = io.Discard
	}
	if rejected {
		*unused = (*unused)[:original]
		if to-from == 1 {
			_, _ = fmt.Fprintln(output, "Not using underapprox of", used[from].Number)
			choices[used[from]] = true
			return nil
		}
		middle := from + (to-from)/2
		if err := d.expansionChoicesRec(unused, used, choices, from, middle); err != nil {
			return err
		}
		return d.expansionChoicesRec(unused, used, choices, middle, to)
	}
	_, _ = fmt.Fprint(output, "Using underapprox of ")
	for _, node := range used[from:to] {
		_, _ = fmt.Fprint(output, node.Number, " ")
		empty, err := node.Map.Underapprox.IsEmpty()
		if err != nil {
			return err
		}
		if empty {
			_, _ = fmt.Fprint(output, "(false!) ")
		}
	}
	_, _ = fmt.Fprintln(output)
	return nil
}

func (d *DerivationTree) ExpansionChoices(best *[]*Node, highPriority, bestOnly bool) error {
	if d == nil || d.tree == nil || d.Heuristic == nil {
		return ErrWrapperOwner
	}
	if !d.Underapprox || d.Constrained || highPriority {
		return d.ExpansionChoicesFull(best, highPriority, bestOnly)
	}
	if d.Top == nil || d.Top.Owner != d.tree {
		return ErrWrapperOwner
	}
	var unused, used []*Node
	choices := make(map[*Node]bool)
	for _, node := range d.leaves {
		empty, err := d.tree.Empty(node)
		if err != nil {
			return err
		}
		if empty {
			unused = append(unused, node)
			continue
		}
		if node.Map == nil {
			return ErrWrapperOwner
		}
		underEmpty, err := node.Map.Underapprox.IsEmpty()
		if err != nil {
			return err
		}
		if d.oldChoices[node] || underEmpty {
			choices[node] = true
		} else {
			used = append(used, node)
		}
	}
	status, err := d.tree.Check(d.Top, unused, nil)
	if err != nil {
		return err
	}
	if status == lbool.False {
		return ErrExpansionChoices
	}
	if err = d.expansionChoicesRec(&unused, used, choices, 0, len(used)); err != nil {
		return err
	}
	d.oldChoices = choices
	var ordered []*Node
	for _, node := range d.leaves {
		if choices[node] {
			ordered = append(ordered, node)
		}
	}
	// Source omits best_only here, using the default midpoint cutoff.
	return d.Heuristic.ChooseExpand(ordered, best, highPriority, false)
}

// ExpandSomeNodes applies source policy dispatch before materialized expansion.
// Stratified original-edge resolution is still supplied by the caller.
func (d *DerivationTree) ExpandSomeNodes(highPriority bool, limit int) (bool, error) {
	var choices []*Node
	if err := d.ExpansionChoices(&choices, highPriority, limit != int(^uint32(0)>>1)); err != nil {
		return false, err
	}
	return d.expandSelected(choices, limit)
}
