package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// addUnderapproxThing appends the source local marker only to conjunctions.
// Source: duality_solver.cpp:1125-1137.
func (u *Unwinding) addUnderapproxThing(fact *Transformer) (result error) {
	m := u.graph.context.manager
	h, err := m.RefGet(fact.formula)
	if err != nil {
		return err
	}
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return nil
	}
	family, err := m.GetFamilyID(h)
	if err != nil {
		return err
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return err
	}
	if family != m.BasicFamilyID() || op != ast.ASTOpAnd {
		return nil
	}
	thing, err := u.graph.context.BoolConst("@thing")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, thing.Close()) }()
	n, err := m.GetNumArgs(h)
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, 0, int(n)+1)
	for i := uint32(0); i < n; i++ {
		arg, err := m.GetArg(h, i)
		if err != nil {
			return err
		}
		args = append(args, arg)
	}
	marker, err := thing.Raw()
	if err != nil {
		return err
	}
	args = append(args, marker)
	combined, err := m.MkAndArgs(args)
	if err != nil {
		return err
	}
	return m.SetRef(fact.formula, combined)
}

// CreateUnderapproxNode captures the current counterexample's lower fact in a
// leaf edge, then resets its annotation to full so it can cover other nodes.
// Its caller registers the underapproximation association and activates it.
// Source: duality_solver.cpp:1139-1151.
func (u *Unwinding) CreateUnderapproxNode(node *Node, counterexample *Counterexample) (*Node, error) {
	if u == nil || node == nil || node.closed || node.Owner != u.graph || node.Map == nil || counterexample == nil {
		return nil, ErrWrapperOwner
	}
	if err := counterexample.open(); err != nil {
		return nil, err
	}
	root := counterexample.Root()
	if root == nil || root.closed || root.Owner.context != u.graph.context {
		return nil, ErrWrapperOwner
	}
	under, err := u.CreateNodeInstance(node.Map, 0)
	if err != nil {
		return nil, err
	}
	if err = under.Annotation.IntersectWith(root.Underapprox); err != nil {
		return nil, err
	}
	if err = u.addUnderapproxThing(under.Annotation); err != nil {
		return nil, err
	}
	edge, err := u.graph.CreateLowerBoundEdge(under)
	if err != nil {
		return nil, err
	}
	if err = under.Annotation.SetFull(); err != nil {
		return nil, err
	}
	if u.backEdges == nil {
		u.backEdges = make(map[*Node]map[*Node]int)
	}
	counts := make(map[*Node]int, len(u.backEdges[node]))
	for original, count := range u.backEdges[node] {
		counts[original] = count
	}
	u.backEdges[under] = counts
	edge.Map = nil
	if err := u.graph.streamReporter.Extend(under); err != nil {
		return nil, err
	}
	return under, nil
}

// ExpandNodeFromOther pushes redirected incoming edges onto the front of the
// candidate queue, preserving source push_front order and repeated positions.
// Source: duality_solver.cpp:731-746.
func (u *Unwinding) ExpandNodeFromOther(node, other *Node) error {
	if u == nil || node == nil || other == nil || node.closed || other.closed || node.Owner != u.graph || other.Owner != u.graph {
		return ErrWrapperOwner
	}
	for _, edge := range other.Incoming {
		if edge == nil || edge.closed || edge.Map == nil || edge.Map.closed || edge.Map.Owner != u.source {
			return ErrWrapperOwner
		}
		children := append([]*Node(nil), edge.Children...)
		for i, child := range children {
			if child == other {
				children[i] = node
			}
		}
		candidate := ExpansionCandidate{Edge: edge.Map, Children: children}
		u.candidates = append(u.candidates, ExpansionCandidate{})
		copy(u.candidates[1:], u.candidates[:len(u.candidates)-1])
		u.candidates[0] = candidate
	}
	return nil
}
