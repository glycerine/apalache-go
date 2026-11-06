package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrNoInductionCandidate = errors.New("duality: no candidate from induction failure")

// CreateEdgesByChildMap indexes source edges once per distinct child while
// preserving source edge order. Rebuild after changing the source graph.
// Source: duality_solver.cpp:564-578.
func (u *Unwinding) CreateEdgesByChildMap() error {
	if u == nil {
		return ErrWrapperOwner
	}
	if err := u.source.open(); err != nil {
		return err
	}
	u.edgesByChild = make(map[*Node][]*Edge)
	for _, edge := range u.source.edges {
		done := make(map[*Node]bool)
		for _, child := range edge.Children {
			if !done[child] {
				u.edgesByChild[child] = append(u.edgesByChild[child], edge)
				done[child] = true
			}
		}
	}
	return nil
}

// ExtractCandidateFromCex implements the active MINIMIZE_CANDIDATES traversal:
// choose the first uncovered active instance selected by the model, or the
// first uncovered instance when the corresponding child is inactive.
// Source: duality_solver.cpp:1338-1363 (MINIMIZE_CANDIDATES_HARDER disabled).
func (u *Unwinding) ExtractCandidateFromCex(edge *Edge, checker *RPFP, root *Node) (candidate ExpansionCandidate, result error) {
	if u == nil || edge == nil || edge.closed || edge.Owner != u.source || checker == nil || root == nil || root.closed || root.Owner != checker || root.Outgoing == nil || len(root.Outgoing.Children) != len(edge.Children) {
		return candidate, ErrWrapperOwner
	}
	candidate.Edge = edge
	for i, original := range edge.Children {
		child := root.Outgoing.Children[i]
		found := false
		for _, instance := range u.covering.instances[original] {
			contains, err := u.covering.Contains(instance)
			if err != nil {
				return candidate, err
			}
			if !contains {
				continue
			}
			selected, err := u.inductionInstanceSelected(checker, child, instance)
			if err != nil {
				return candidate, err
			}
			if selected {
				candidate.Children = append(candidate.Children, instance)
				found = true
				break
			}
		}
		if !found {
			return candidate, ErrNoInductionCandidate
		}
	}
	return candidate, nil
}
func (u *Unwinding) inductionInstanceSelected(checker *RPFP, child, instance *Node) (selected bool, result error) {
	empty, err := checker.Empty(child)
	if err != nil || empty {
		return empty, err
	}
	other := (*Node)(nil)
	if child.Outgoing == nil {
		other = child
	}
	marker, err := NodeMarker(instance, other)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, marker.Close()) }()
	var value *Expr
	if child.Outgoing != nil {
		value, err = checker.Eval(child.Outgoing, marker)
	} else {
		value, err = checker.evaluate(marker)
	}
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, value.Close()) }()
	h, err := value.Raw()
	if err != nil {
		return false, err
	}
	return checker.context.manager.IsTrue(h), nil
}

// ExpandNode activates an instance and appends source-order candidates from
// induction failures that use it. Temporary checker graphs/scopes own their
// assertions and models; queued candidates borrow only unwinding nodes.
// Source: duality_solver.cpp:698-726, active NEW_EXPAND.
func (u *Unwinding) ExpandNode(node *Node, native solver.Solver, evaluator model.ModelEvaluatorConfig, simplifier *interp.Manager) (result error) {
	if u == nil || node == nil || node.closed || node.Owner != u.graph || node.Map == nil || native == nil || u.edgesByChild == nil {
		return ErrWrapperOwner
	}
	timer, err := startProfile(u.graph.context, "GenCandIndFailUsing")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	if err := u.ActivateInstance(node); err != nil {
		return err
	}
	for _, edge := range u.edgesByChild[node.Map] {
		if err := u.expandInductionEdge(node, edge, native, evaluator, simplifier); err != nil {
			return err
		}
	}
	return nil
}
func (u *Unwinding) expandInductionEdge(node *Node, edge *Edge, native solver.Solver, evaluator model.ModelEvaluatorConfig, simplifier *interp.Manager) (result error) {
	checker, err := NewRPFPWithVocabulary(u.source.vocabulary)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, checker.Close()) }()
	if err = checker.BindSolver(native); err != nil {
		return err
	}
	if err = checker.BindModelEvaluator(evaluator); err != nil {
		return err
	}
	if err = checker.Push(); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, checker.Pop(1)) }()
	root, err := u.CheckerJustForEdge(edge, checker, true, simplifier)
	if err != nil || root == nil {
		return err
	}
	using, err := u.graph.context.BoolVal(false)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, using.Close()) }()
	for i, original := range edge.Children {
		if original != node.Map {
			continue
		}
		if err = addInductionUsingMarker(checker, root.Outgoing.Children[i].Outgoing, node, using); err != nil {
			return err
		}
	}
	h, err := using.Raw()
	if err != nil {
		return err
	}
	if err = native.Assert(h); err != nil {
		return err
	}
	status, err := checker.Check(root, nil, nil)
	if err != nil || status == lbool.False {
		return err
	}
	candidate, err := u.ExtractCandidateFromCex(edge, checker, root)
	if err != nil {
		return err
	}
	if err := u.graph.streamReporter.InductionFailure(edge, candidate.Children); err != nil {
		return err
	}
	u.candidates = append(u.candidates, candidate)
	return nil
}
func addInductionUsingMarker(checker *RPFP, lower *Edge, node *Node, using *Expr) (result error) {
	marker, err := NodeMarker(node, nil)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, marker.Close()) }()
	localized, err := checker.Localize(lower, marker)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, localized.Close()) }()
	next, err := using.Or(localized)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, next.Close()) }()
	h, err := next.Raw()
	if err != nil {
		return err
	}
	return checker.context.manager.SetRef(using.value.ref, h)
}
