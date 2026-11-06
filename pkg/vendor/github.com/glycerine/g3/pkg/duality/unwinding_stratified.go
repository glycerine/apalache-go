package duality

import "errors"

var ErrLevelizedUnwinding = errors.New("duality: internal error in levelized unwinding")

// Extend supplies fresh query graphs for the concrete outer-search extension
// and releases unused graphs. Only the counterexample owns transferred graphs.
func (s *UnwindingSearch) Extend(candidate ExpansionCandidate) (node *Node, satisfied bool, result error) {
	bounds, err := s.newDerivation()
	if err != nil {
		return nil, false, err
	}
	defer func() {
		graph := bounds.Search.tree
		if s.Counterexample == nil || s.Counterexample.Tree() != graph {
			result = errors.Join(result, graph.Close())
		}
	}()
	var completion *DerivationSearch
	if s.UseUnderapprox {
		completion, err = s.newDerivation()
		if err != nil {
			return nil, false, err
		}
		defer func() {
			graph := completion.Search.tree
			if s.Counterexample == nil || s.Counterexample.Tree() != graph {
				result = errors.Join(result, graph.Close())
			}
		}()
	}
	services := &ExtensionSearch{Bounds: bounds, Completion: completion, Counterexample: s.Counterexample, Proposers: s.Proposers, UseUnderapprox: s.UseUnderapprox}
	return s.Unwinding.Extend(candidate, services)
}

// DoStratifiedInlining builds the active source's single initial unfolding
// layer. Only back-edge targets use shared level-zero false leaves; forward
// dependencies reuse already built level-one instances. The attempt flag is
// set before work, including counterexample/error exits, exactly as in source.
// Source: duality_solver.cpp:941-972, NEW_STRATIFIED_INLINING enabled.
func (s *UnwindingSearch) DoStratifiedInlining() (satisfied bool, result error) {
	if s == nil || s.Unwinding == nil {
		return false, ErrWrapperOwner
	}
	u := s.Unwinding
	if u.stratifiedDone {
		return true, nil
	}
	u.stratifiedDone = true
	if err := u.DoTopoSort(); err != nil {
		return false, err
	}
	levels := [2]map[*Node]*Node{make(map[*Node]*Node), make(map[*Node]*Node)}
	for _, edge := range u.sortedEdges {
		if err := u.source.checkpoint(); err != nil {
			return false, err
		}
		if edge == nil || edge.closed {
			return false, ErrLevelizedUnwinding
		}
		children := make([]*Node, len(edge.Children))
		for i, child := range edge.Children {
			level := 1
			if u.topological[child] >= u.topological[edge.Parent] {
				level = 0
			}
			instance := levels[level][child]
			if instance == nil {
				if level != 0 {
					return false, ErrLevelizedUnwinding
				}
				var err error
				instance, err = u.CreateLeaf(child)
				if err != nil {
					return false, err
				}
				levels[0][child] = instance
			}
			children[i] = instance
		}
		node, ok, err := s.Extend(ExpansionCandidate{Edge: edge, Children: children})
		if err != nil {
			return false, err
		}
		if err = u.MarkExpanded(node); err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		levels[1][edge.Parent] = node
	}
	return true, nil
}
