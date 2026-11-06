package duality

// Source: duality_solver.cpp:499-515,692-700.
import "sort"

// Unwinding owns outer-search bookkeeping over a borrowed graph and covering
// relation. Node and edge AST ownership belongs to the graph.
type Unwinding struct {
	recursionBounded   bool
	stratifiedDone     bool
	toExpand           []*Node
	postponed          []ExpansionCandidate
	leaves             []*Node
	lastDecisions      int
	edgesByChild       map[*Node][]*Edge
	overapproximations map[*Node]bool
	updatedNodes       map[*Node]bool
	topological        map[*Node]int
	sortedEdges        []*Edge
	FeasibleEdges      bool
	RecursionBound     int
	backEdges          map[*Node]map[*Node]int
	StratifiedInlining bool
	leafMap            map[*Node]*Node
	candidates         []ExpansionCandidate
	source, graph      *RPFP
	covering           *Covering
	unexpanded         []*Node
}

func NewUnwinding(source, graph *RPFP, covering *Covering) (*Unwinding, error) {
	if err := source.open(); err != nil {
		return nil, err
	}
	if err := graph.open(); err != nil {
		return nil, err
	}
	if source.context != graph.context {
		return nil, ErrContextMismatch
	}
	if covering == nil || covering.graph != graph {
		return nil, ErrWrapperOwner
	}
	return &Unwinding{source: source, graph: graph, covering: covering, FeasibleEdges: true, RecursionBound: -1, backEdges: make(map[*Node]map[*Node]int)}, nil
}
func (u *Unwinding) CreateNodeInstance(node *Node, number int) (*Node, error) {
	if u == nil || node == nil || node.Owner != u.source {
		return nil, ErrWrapperOwner
	}
	instance, err := u.graph.CloneNode(node)
	if err != nil {
		return nil, err
	}
	if err = instance.Annotation.SetFull(); err != nil {
		return nil, err
	}
	if number < 0 {
		instance.Number = number
	}
	// Source lnode compares only numbers, so equal-number keys are equivalent.
	pos := sort.Search(len(u.unexpanded), func(n int) bool { return u.unexpanded[n].Number >= instance.Number })
	if pos == len(u.unexpanded) || u.unexpanded[pos].Number != instance.Number {
		u.unexpanded = append(u.unexpanded, nil)
		copy(u.unexpanded[pos+1:], u.unexpanded[pos:])
		u.unexpanded[pos] = instance
	}
	if err = u.covering.RegisterInstance(instance); err != nil {
		return nil, err
	}
	if err = u.covering.SetInstanceActive(instance, false); err != nil {
		return nil, err
	}
	return instance, nil
}
func (u *Unwinding) CreateEdgeInstance(source *Edge, parent *Node, children []*Node) (*Edge, error) {
	if u == nil || source == nil || source.Owner != u.source || source.closed {
		return nil, ErrWrapperOwner
	}
	edge, err := u.graph.CreateEdge(parent, source.F, children)
	if err != nil {
		return nil, err
	}
	edge.Map = source
	return edge, nil
}
func (u *Unwinding) PendingNodes() []*Node { return append([]*Node(nil), u.unexpanded...) }

// ActivateInstance performs the source queue removal and active-instance
// insertion before candidate generation, which is a separate search step.
func (u *Unwinding) ActivateInstance(node *Node) error {
	if u == nil || node == nil || node.Owner != u.graph || node.closed {
		return ErrWrapperOwner
	}
	for n, other := range u.unexpanded {
		if other.Number == node.Number {
			u.unexpanded = append(u.unexpanded[:n], u.unexpanded[n+1:]...)
			break
		}
	}
	return u.covering.SetInstanceActive(node, true)
}
