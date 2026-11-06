package duality

// Source: duality_solver.cpp:845-857,867-900,1700-1714.
import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (u *Unwinding) NodePastRecursionBound(node *Node) (bool, error) {
	return u.nodePastRecursionBound(node, false)
}

// nodePastRecursionBound optionally publishes the source's final-result
// cutoff diagnostic. Candidate filtering uses the silent form; only
// TestRecursionBounded reports a cutoff that can affect the published result.
func (u *Unwinding) nodePastRecursionBound(node *Node, report bool) (bool, error) {
	if u == nil || node == nil || node.Owner != u.graph || node.closed {
		return false, ErrWrapperOwner
	}
	if u.RecursionBound < 0 {
		return false, nil
	}
	for original, count := range u.backEdges[node] {
		if uint64(count) > uint64(original.RecursionBound) {
			if report && u.graph.streamReporter != nil {
				name, err := u.graph.streamReporter.nodeName(original)
				if err != nil {
					return false, err
				}
				if err := u.graph.streamReporter.Message(fmt.Sprintf("cut off %s at depth %d", name, count)); err != nil {
					return false, err
				}
			}
			return true, nil
		}
	}
	return false, nil
}

// UpdateBackEdges uses the source max-over-children recurrence and increments
// a child's original-node counter on a nondecreasing topological edge.
func (u *Unwinding) UpdateBackEdges(node *Node, order map[*Node]int) error {
	if u == nil || node == nil || node.Owner != u.graph || node.Outgoing == nil {
		return ErrWrapperOwner
	}
	if u.backEdges[node] == nil {
		u.backEdges[node] = make(map[*Node]int)
	}
	for _, child := range node.Outgoing.Children {
		counts := make(map[*Node]int)
		for original, count := range u.backEdges[child] {
			counts[original] = count
		}
		if order[child.Map] >= order[node.Map] {
			counts[child.Map]++
		}
		for original, count := range counts {
			if count > u.backEdges[node][original] {
				u.backEdges[node][original] = count
			}
		}
	}
	return nil
}
func (u *Unwinding) CandidateFeasible(candidate ExpansionCandidate, target solver.Solver, simplifier *interp.Manager) (_ bool, result error) {
	if u == nil {
		return false, ErrWrapperOwner
	}
	if !u.FeasibleEdges {
		return true, nil
	}
	if candidate.Edge == nil || candidate.Edge.Owner != u.source || candidate.Edge.closed {
		return false, ErrWrapperOwner
	}
	timer, err := startProfile(u.graph.context, "CandidateFeasible")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	checker, err := NewRPFPWithVocabulary(u.graph.vocabulary)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, checker.Close()) }()
	if err = checker.BindSolver(target); err != nil {
		return false, err
	}
	if err = checker.Push(); err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, checker.Pop(1)) }()
	root, err := checker.CloneNode(candidate.Edge.Parent)
	if err != nil {
		return false, err
	}
	for _, child := range candidate.Children {
		past, err := u.NodePastRecursionBound(child)
		if err != nil {
			return false, err
		}
		if past {
			return false, nil
		}
	}
	if err = u.GenNodeSolutionFromIndSet(candidate.Edge.Parent, root.Bound, false, simplifier); err != nil {
		return false, err
	}
	if err = checker.AssertNode(root); err != nil {
		return false, err
	}
	children := make([]*Node, len(candidate.Children))
	for n, child := range candidate.Children {
		children[n], err = checker.CloneNode(child)
		if err != nil {
			return false, err
		}
	}
	edge, err := checker.CreateEdge(root, candidate.Edge.F, children)
	if err != nil {
		return false, err
	}
	if err = checker.AssertEdge(edge, 0, true, false); err != nil {
		return false, err
	}
	status, err := checker.Check(root, nil, nil)
	if err == nil && status == lbool.False {
		err = u.graph.streamReporter.Reject(candidate.Edge, candidate.Children)
	}
	return status != lbool.False, err
}
