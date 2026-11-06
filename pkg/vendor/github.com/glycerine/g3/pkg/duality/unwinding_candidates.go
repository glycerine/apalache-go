package duality

// Source: duality_solver.cpp:619-662,1112-1113.
import "github.com/glycerine/g3/pkg/ast"

type ExpansionCandidate struct {
	Edge     *Edge
	Children []*Node
}

func (u *Unwinding) RegisterStratifiedLeaf(source, instance *Node) error {
	if u == nil || source == nil || instance == nil || source.Owner != u.source || instance.Owner != u.graph || source.closed || instance.closed {
		return ErrWrapperOwner
	}
	if u.leafMap == nil {
		u.leafMap = make(map[*Node]*Node)
	}
	u.leafMap[source] = instance
	return nil
}
func (u *Unwinding) ProducedBySI(edge *Edge, children []*Node) (bool, error) {
	if u == nil || edge == nil || edge.closed || edge.Owner != u.source {
		return false, ErrWrapperOwner
	}
	other, found := u.leafMap[edge.Parent]
	if !found {
		return false, nil
	}
	if other.Outgoing == nil {
		return false, ErrWrapperOwner
	}
	if other.Outgoing.Map != edge {
		return false, nil
	}
	if len(children) > len(other.Outgoing.Children) {
		return false, ast.ErrASTDomain
	}
	for n, child := range children {
		if other.Outgoing.Children[n] != child {
			return false, nil
		}
	}
	return true, nil
}
func (u *Unwinding) AddCandidate(edge *Edge, children []*Node) error {
	if u == nil || edge == nil || edge.Owner != u.source || edge.closed {
		return ErrWrapperOwner
	}
	for _, child := range children {
		if child == nil || child.Owner != u.graph || child.closed {
			return ErrWrapperOwner
		}
	}
	if u.StratifiedInlining {
		produced, err := u.ProducedBySI(edge, children)
		if err != nil {
			return err
		}
		if produced {
			return nil
		}
	}
	u.candidates = append(u.candidates, ExpansionCandidate{Edge: edge, Children: append([]*Node(nil), children...)})
	return nil
}

// GenCandidates preserves source position-major Cartesian-product order.
// The source overload ignores its pos argument and starts recursion at zero.
func (u *Unwinding) GenCandidates(pos int, edge *Edge, positions [][]*Node) error {
	children := make([]*Node, len(positions))
	var generate func(int) error
	generate = func(position int) error {
		if position == len(positions) {
			return u.AddCandidate(edge, children)
		}
		for _, node := range positions[position] {
			children[position] = node
			if err := generate(position + 1); err != nil {
				return err
			}
		}
		return nil
	}
	return generate(0)
}
func (u *Unwinding) NextCandidate() (ExpansionCandidate, bool) {
	if u == nil || len(u.candidates) == 0 {
		return ExpansionCandidate{}, false
	}
	next := u.candidates[0]
	u.candidates[0] = ExpansionCandidate{}
	u.candidates = u.candidates[1:]
	return next, true
}
