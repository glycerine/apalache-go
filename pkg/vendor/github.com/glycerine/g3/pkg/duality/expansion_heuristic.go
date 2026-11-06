package duality

// Source: duality_solver.cpp:200-266, active Heuristic implementation.

// ExpansionHeuristic counts updates to original nodes. Candidate instances
// share the score of their Map node, as in the source derivation search.
type ExpansionHeuristic struct {
	replay  *counterexampleReplay
	graph   *RPFP
	updates map[*Node]int
}

func NewExpansionHeuristic(graph *RPFP) (*ExpansionHeuristic, error) {
	if err := graph.open(); err != nil {
		return nil, err
	}
	return &ExpansionHeuristic{graph: graph, updates: make(map[*Node]int)}, nil
}
func (h *ExpansionHeuristic) Update(node *Node) error {
	if h == nil || h.graph == nil {
		return ErrWrapperOwner
	}
	if err := h.graph.open(); err != nil {
		return err
	}
	if node == nil || node.closed {
		return ErrWrapperOwner
	}
	h.updates[node]++
	return nil
}

// ChooseExpand inserts qualifying instances into best without removing prior
// entries. The default cutoff is halfway between the best and worst score;
// bestOnly narrows it to the minimum. High-priority calls make no choices.
// Slice order is supplied by the caller; scores never depend on map iteration.
func (h *ExpansionHeuristic) ChooseExpand(choices []*Node, best *[]*Node, highPriority, bestOnly bool) error {
	if h == nil || h.graph == nil {
		return ErrWrapperOwner
	}
	if err := h.graph.open(); err != nil {
		return err
	}
	if h.replay != nil {
		return h.chooseReplay(choices, best, highPriority)
	}
	return h.chooseBase(choices, best, highPriority, bestOnly)
}

func (h *ExpansionHeuristic) chooseBase(choices []*Node, best *[]*Node, highPriority, bestOnly bool) error {
	if highPriority {
		return nil
	}
	if best == nil {
		return ErrWrapperOwner
	}
	minimum, maximum := int(^uint(0)>>1), 0
	for _, node := range choices {
		if node == nil || node.closed {
			return ErrWrapperOwner
		}
		score := h.updates[node.Map]
		if score < minimum {
			minimum = score
		}
		if score > maximum {
			maximum = score
		}
	}
	cutoff := minimum
	if !bestOnly {
		cutoff = minimum + (maximum-minimum)/2
	}
	seen := make(map[*Node]bool)
	for _, node := range *best {
		seen[node] = true
	}
	for _, node := range choices {
		if !seen[node] && h.updates[node.Map] <= cutoff {
			*best = append(*best, node)
			seen[node] = true
		}
	}
	return nil
}

// Done and UseNode implement the source base heuristic's neutral hooks.
func (h *ExpansionHeuristic) Done() error {
	if h != nil && h.replay != nil {
		clear(h.replay.matches)
		return h.replay.previous.Clear()
	}
	return nil
}

// restoreReplay returns an unconsumed guide before an interrupted search
// releases its heuristic. Any partial replacement result moves into the
// heuristic and is discarded by Close.
func (h *ExpansionHeuristic) restoreReplay(target *Counterexample) error {
	if h == nil || h.replay == nil {
		return nil
	}
	if target == nil {
		return ErrWrapperOwner
	}
	return target.Swap(h.replay.previous)
}

// Close releases a replay counterexample after interrupted searches as well.
func (h *ExpansionHeuristic) Close() error           { return h.Done() }
func (h *ExpansionHeuristic) UseNode(node *Node) int { return 0 }
