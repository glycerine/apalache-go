package duality

// Source: duality_solver.cpp:2144-2154,2425-2447,2490-2513,2555-2566.
import "errors"

var ErrUnmapNode = errors.New("can't unmap node")
var ErrUpdatedExpansionChild = errors.New("help!")

type derivationLevel struct {
	level      int
	expansions []*Node
	cleanup    []derivationExpansionCleanup
	popped     bool
}

type derivationExpansionCleanup struct {
	parent   *Node
	edge     *Edge
	children []*Node
}

// BacktrackingDerivation tracks the source slow-search expansion levels over
// a borrowed derivation tree. Solver scopes are pushed by its search caller.
type BacktrackingDerivation struct {
	sourceCovering *Covering
	*DerivationTree
	stack        []derivationLevel
	nodeMap      map[*Node][]*Node
	updatedNodes []*Node
}

func NewBacktrackingDerivation(tree *DerivationTree) (*BacktrackingDerivation, error) {
	if tree == nil || tree.tree == nil {
		return nil, ErrWrapperOwner
	}
	if err := tree.tree.open(); err != nil {
		return nil, err
	}
	return &BacktrackingDerivation{DerivationTree: tree, stack: []derivationLevel{{}}, nodeMap: make(map[*Node][]*Node)}, nil
}
func (d *DerivationTree) RemoveLeaves(remove map[*Node]bool) {
	previous := d.leaves
	d.leaves = nil
	for _, node := range previous {
		if !remove[node] {
			d.leaves = append(d.leaves, node)
		}
	}
}
func (d *DerivationTree) RemoveExpansion(node *Node) error {
	if node == nil || node.Owner != d.tree || node.Outgoing == nil {
		return ErrWrapperOwner
	}
	edge := node.Outgoing
	parent := edge.Parent
	children := append([]*Node(nil), edge.Children...)
	if err := d.tree.DeleteEdge(edge); err != nil {
		return err
	}
	for _, child := range children {
		if err := d.tree.DeleteNode(child); err != nil {
			return err
		}
	}
	d.leaves = append(d.leaves, parent)
	return nil
}

// ExpandMaterializedNode records the level before invoking the base expansion.
// As in source ExpandSomeNodes, the caller consumes the selected leaf first.
func (d *BacktrackingDerivation) ExpandMaterializedNode(node *Node) error {
	if d == nil || d.DerivationTree == nil {
		return ErrWrapperOwner
	}
	if err := d.tree.solverReady(); err != nil {
		return err
	}
	level, err := d.tree.solver.ScopeLevel()
	if err != nil {
		return err
	}
	d.stack = append(d.stack, derivationLevel{level: int(level), expansions: []*Node{node}})
	if err = d.DerivationTree.ExpandMaterializedNode(node); err != nil {
		return err
	}
	for _, child := range node.Outgoing.Children {
		d.nodeMap[child.Map] = append(d.nodeMap[child.Map], child)
	}
	return nil
}
func (d *BacktrackingDerivation) UnmapNode(node *Node) error {
	if d == nil || node == nil {
		return ErrWrapperOwner
	}
	instances := d.nodeMap[node.Map]
	for n, candidate := range instances {
		if candidate == node {
			instances[n] = instances[len(instances)-1]
			d.nodeMap[node.Map] = instances[:len(instances)-1]
			return nil
		}
	}
	return ErrUnmapNode
}
func (d *BacktrackingDerivation) PopLevel() error {
	if d == nil || len(d.stack) < 2 {
		return ErrWrapperOwner
	}
	level := &d.stack[len(d.stack)-1]
	if level.cleanup == nil {
		required := make(map[*Node]int)
		updated := make(map[*Node]bool, len(d.updatedNodes))
		for _, node := range d.updatedNodes {
			updated[node] = true
		}
		for _, node := range level.expansions {
			if node == nil || node.Outgoing == nil {
				return ErrWrapperOwner
			}
			for _, child := range node.Outgoing.Children {
				if updated[child] {
					return ErrUpdatedExpansionChild
				}
				required[child]++
			}
		}
		for child, count := range required {
			available := 0
			for _, candidate := range d.nodeMap[child.Map] {
				if candidate == child {
					available++
				}
			}
			if available < count {
				return ErrUnmapNode
			}
		}
		for _, node := range level.expansions {
			level.cleanup = append(level.cleanup, derivationExpansionCleanup{
				parent: node, edge: node.Outgoing, children: append([]*Node(nil), node.Outgoing.Children...),
			})
		}
	}
	if !level.popped {
		if err := d.tree.Pop(1); err != nil {
			return err
		}
		level.popped = true
	}
	for _, item := range level.cleanup {
		if !item.edge.closed {
			if err := d.tree.DeleteEdge(item.edge); err != nil {
				return err
			}
		}
		for _, child := range item.children {
			if !child.closed {
				if err := d.tree.DeleteNode(child); err != nil {
					return err
				}
			}
		}
	}
	remove := make(map[*Node]bool)
	for _, item := range level.cleanup {
		for _, child := range item.children {
			remove[child] = true
			if err := d.UnmapNode(child); err != nil {
				return err
			}
		}
	}
	d.RemoveLeaves(remove)
	for _, item := range level.cleanup {
		d.leaves = append(d.leaves, item.parent)
	}
	d.stack = d.stack[:len(d.stack)-1]
	return nil
}

// LevelUsedInProof must be queried before deleting that level's expansions.
// The proof core may outlive native solver scopes while backtracking.
// Source: duality_solver.cpp:2469-2476.
func (d *BacktrackingDerivation) LevelUsedInProof(level int) (bool, error) {
	if d == nil || d.DerivationTree == nil || level < 0 || level >= len(d.stack) {
		return false, ErrWrapperOwner
	}
	for _, node := range d.stack[level].expansions {
		if node == nil {
			return false, ErrWrapperOwner
		}
		used, err := d.tree.EdgeUsedInProof(node.Outgoing)
		if err != nil || used {
			return used, err
		}
	}
	return false, nil
}
