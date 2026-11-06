package duality

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

var ErrReplayMapping = errors.New("duality: catastrophe in ReplayHeuristic::ChooseExpand")

type counterexampleReplay struct {
	previous *Counterexample
	matches  map[*Node]*Node
}

// NewReplayExpansionHeuristic takes the previous counterexample by swapping
// ownership from the caller. Only active ChooseExpand/MatchNode/Done behavior
// is ported; source UseNode calls are disabled in the slow search.
// Source: duality_solver.cpp:2982-3084, active ReplayHeuristic consumers.
func NewReplayExpansionHeuristic(graph *RPFP, previous *Counterexample) (*ExpansionHeuristic, error) {
	h, err := NewExpansionHeuristic(graph)
	if err != nil {
		return nil, err
	}
	if previous == nil {
		return nil, ErrWrapperOwner
	}
	owned := NewCounterexample()
	if err = owned.Swap(previous); err != nil {
		return nil, err
	}
	h.replay = &counterexampleReplay{previous: owned, matches: make(map[*Node]*Node)}
	return h, nil
}

func replayNodeBaseName(node *Node) (string, error) {
	name, err := replayNodeName(node)
	if err != nil {
		return "", err
	}
	if pos := strings.Index(name, "@@"); pos > 0 {
		return name[:pos], nil
	}
	return name, nil
}

func replayNodeName(node *Node) (string, error) {
	if node == nil || node.closed {
		return "", ErrWrapperOwner
	}
	m := node.Owner.context.manager
	decl, err := m.RefGet(node.Name)
	if err != nil {
		return "", err
	}
	name, err := m.GetASTName(decl)
	if err != nil {
		return "", err
	}
	symbol, err := newSymbol(node.Owner.context, name)
	if err != nil {
		return "", err
	}
	text, err := symbol.String()
	if err != nil {
		return "", err
	}
	return text, nil
}

func (r *counterexampleReplay) match(node *Node) (*Node, error) {
	if node == nil || node.closed {
		return nil, ErrWrapperOwner
	}
	if old, found := r.matches[node]; found {
		return old, nil
	}
	if len(node.Incoming) == 0 || node.Incoming[0] == nil {
		return nil, ErrReplayMapping
	}
	parent := node.Incoming[0].Parent
	oldParent, found := r.matches[parent]
	if !found || parent.Outgoing == nil {
		return nil, ErrReplayMapping
	}
	children := parent.Outgoing.Children
	if oldParent != nil && oldParent.Outgoing != nil {
		oldChildren := oldParent.Outgoing.Children
		j := 0
		for _, child := range children {
			name, err := replayNodeBaseName(child)
			if err != nil {
				return nil, err
			}
			if j < len(oldChildren) {
				oldName, err := replayNodeBaseName(oldChildren[j])
				if err != nil {
					return nil, err
				}
				if name == oldName {
					r.matches[child] = oldChildren[j]
					j++
					continue
				}
			}
			childName, err := replayNodeName(child)
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(os.Stderr, "WARNING: duality: unmatched child: %s\n", childName)
			r.matches[child] = nil
		}
	} else {
		for _, child := range children {
			r.matches[child] = nil
		}
	}
	return r.matches[node], nil
}

func (h *ExpansionHeuristic) chooseReplay(choices []*Node, best *[]*Node, highPriority bool) error {
	if best == nil {
		return ErrWrapperOwner
	}
	r := h.replay
	if len(choices) == 0 {
		return h.chooseBase(choices, best, false, false)
	}
	if len(r.matches) == 0 {
		r.matches[choices[0]] = r.previous.Root()
	}
	// Source deliberately drops best_only when falling back to its base class.
	if !highPriority || r.previous.Tree() == nil {
		return h.chooseBase(choices, best, false, false)
	}
	var matched []*Node
	for _, node := range choices {
		old, err := r.match(node)
		if err != nil {
			return err
		}
		if old == nil {
			continue
		}
		empty, err := r.previous.Tree().Empty(old)
		if err != nil {
			return err
		}
		if !empty {
			matched = append(matched, node)
		}
	}
	return h.chooseBase(matched, best, false, false)
}
