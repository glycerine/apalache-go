package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
)

// GenNodeSolutionWithMarkers builds conjunctions of marker-implies-annotation
// clauses and requires at least one marker. Source reports an available
// uncovered instance even if recursion-bound filtering later excludes it.
// Source: duality_solver.cpp:1236-1261.
func (u *Unwinding) GenNodeSolutionWithMarkers(node *Node, destination *Transformer, expandedOnly bool, other *Node, simplifier *interp.Manager) (available bool, result error) {
	if u == nil || node == nil || node.closed || node.Owner != u.source || simplifier == nil {
		return false, ErrWrapperOwner
	}
	if err := destination.open(); err != nil {
		return false, err
	}
	if destination.context != u.graph.context {
		return false, ErrContextMismatch
	}
	m, err := simplifier.ASTManager()
	if err != nil {
		return false, err
	}
	if m != u.graph.context.manager {
		return false, ErrContextMismatch
	}
	if err = destination.SetFull(); err != nil {
		return false, err
	}
	disjunction, err := u.graph.context.BoolVal(false)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, disjunction.Close()) }()
	instances := u.covering.allOfNode[node]
	if expandedOnly {
		instances = u.covering.instances[node]
	}
	for _, instance := range instances {
		contains, err := u.covering.Contains(instance)
		if err != nil {
			return false, err
		}
		if !contains {
			continue
		}
		available = true
		past, err := u.NodePastRecursionBound(instance)
		if err != nil {
			return false, err
		}
		if past {
			continue
		}
		if err = u.addMarkedNodeSolution(instance, destination, disjunction, other); err != nil {
			return false, err
		}
	}
	formula, err := destination.Formula()
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	combined, err := formula.And(disjunction)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, combined.Close()) }()
	h, err := combined.Raw()
	if err != nil {
		return false, err
	}
	simplified, err := simplifier.Simplify(h)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&simplified)) }()
	h, err = m.RefGet(simplified)
	if err != nil {
		return false, err
	}
	return available, m.SetRef(destination.formula, h)
}

func (u *Unwinding) addMarkedNodeSolution(instance *Node, destination *Transformer, disjunction *Expr, other *Node) (result error) {
	temporary, err := instance.Annotation.Clone()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, temporary.Close()) }()
	marker, err := NodeMarker(instance, other)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, marker.Close()) }()
	negative, err := marker.Not()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, negative.Close()) }()
	formula, err := temporary.Formula()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	guarded, err := negative.Or(formula)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, guarded.Close()) }()
	m := u.graph.context.manager
	h, err := guarded.Raw()
	if err != nil {
		return err
	}
	if err = m.SetRef(temporary.formula, h); err != nil {
		return err
	}
	if err = destination.IntersectWith(temporary); err != nil {
		return err
	}
	next, err := disjunction.Or(marker)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, next.Close()) }()
	h, err = next.Raw()
	if err != nil {
		return err
	}
	return m.SetRef(disjunction.value.ref, h)
}

// CheckerJustForEdge materializes the source induction-failure query in a
// caller-owned, scoped checker graph. A nil root is the source early return:
// the parent solution is full or some child has no eligible instance.
// Source: duality_solver.cpp:1264-1285.
func (u *Unwinding) CheckerJustForEdge(edge *Edge, checker *RPFP, expandedOnly bool, simplifier *interp.Manager) (*Node, error) {
	if u == nil || edge == nil || edge.closed || edge.Owner != u.source || checker == nil {
		return nil, ErrWrapperOwner
	}
	if err := checker.solverReady(); err != nil {
		return nil, err
	}
	if checker.context != u.graph.context {
		return nil, ErrContextMismatch
	}
	root, err := checker.CloneNode(edge.Parent)
	if err != nil {
		return nil, err
	}
	if err = u.GenNodeSolutionFromIndSet(edge.Parent, root.Bound, false, simplifier); err != nil {
		return nil, err
	}
	full, err := root.Bound.IsFull()
	if err != nil || full {
		return nil, err
	}
	if err = checker.AssertNode(root); err != nil {
		return nil, err
	}
	children := make([]*Node, 0, len(edge.Children))
	for _, original := range edge.Children {
		child, err := checker.CloneNode(original)
		if err != nil {
			return nil, err
		}
		available, err := u.GenNodeSolutionWithMarkers(original, child.Annotation, expandedOnly, nil, simplifier)
		if err != nil || !available {
			return nil, err
		}
		lower, err := checker.CreateLowerBoundEdge(child)
		if err != nil {
			return nil, err
		}
		if err = checker.AssertEdge(lower, 0, false, false); err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	copy, err := checker.CreateEdge(root, edge.F, children)
	if err != nil {
		return nil, err
	}
	if err = checker.AssertEdge(copy, 0, false, false); err != nil {
		return nil, err
	}
	return root, nil
}
