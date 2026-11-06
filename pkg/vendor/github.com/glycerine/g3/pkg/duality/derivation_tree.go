package duality

// Source: duality_solver.cpp:1910-1928,1986-2013.

import (
	"errors"
	"io"
	"os"
)

// DerivationTree owns search bookkeeping over a borrowed working graph. The
// graph owns all created instances and edges. Original nodes are borrowed.
type DerivationTree struct {
	Top                                  *Node
	Constrained                          bool
	Output                               io.Writer
	oldChoices                           map[*Node]bool
	Heuristic                            *ExpansionHeuristic
	original, tree                       *RPFP
	leaves                               []*Node
	FullExpand, Underapprox, FalseApprox bool
}

func NewDerivationTree(original, tree *RPFP) (*DerivationTree, error) {
	if err := original.open(); err != nil {
		return nil, err
	}
	if err := tree.open(); err != nil {
		return nil, err
	}
	if original.context != tree.context {
		return nil, ErrContextMismatch
	}
	heuristic, err := NewExpansionHeuristic(original)
	if err != nil {
		return nil, err
	}
	return &DerivationTree{original: original, tree: tree, FalseApprox: true, Heuristic: heuristic, Output: os.Stdout}, nil
}
func (d *DerivationTree) initializeApproximatedInstance(node *Node) error {
	if node == nil || node.Owner != d.tree || node.closed || node.Map == nil || node.Map.closed {
		return ErrWrapperOwner
	}
	annotation, err := node.Map.Annotation.Clone()
	if err != nil {
		return err
	}
	if err = node.Annotation.Close(); err != nil {
		return errors.Join(err, annotation.Close())
	}
	node.Annotation = annotation
	d.leaves = append(d.leaves, node)
	return nil
}
func (d *DerivationTree) CreateApproximatedInstance(original *Node) (*Node, error) {
	if d == nil || d.tree == nil {
		return nil, ErrWrapperOwner
	}
	if original == nil || original.Owner != d.original {
		return nil, ErrWrapperOwner
	}
	node, err := d.tree.CloneNode(original)
	if err != nil {
		return nil, err
	}
	if err = d.initializeApproximatedInstance(node); err != nil {
		return nil, err
	}
	return node, nil
}

// ExpandMaterializedNode performs the source ExpandNode step after the
// original node's outgoing edge has been resolved. Stratified inlining must
// materialize that original edge before calling this operation.
func (d *DerivationTree) ExpandMaterializedNode(node *Node) error {
	if d == nil || d.tree == nil {
		return ErrWrapperOwner
	}
	if err := d.tree.solverReady(); err != nil {
		return err
	}
	if node == nil || node.Owner != d.tree || node.closed || node.Map == nil {
		return ErrWrapperOwner
	}
	edge := node.Outgoing
	if edge != nil {
		for _, child := range edge.Children {
			if err := d.initializeApproximatedInstance(child); err != nil {
				return err
			}
		}
	} else {
		source := node.Map.Outgoing
		if source == nil || source.closed {
			return ErrWrapperOwner
		}
		children := make([]*Node, len(source.Children))
		for n, child := range source.Children {
			instance, err := d.CreateApproximatedInstance(child)
			if err != nil {
				return err
			}
			children[n] = instance
		}
		var err error
		edge, err = d.tree.CreateEdge(node, source.F, children)
		if err != nil {
			return err
		}
		edge.Map = source.Map
	}
	if err := d.tree.AssertEdge(edge, 0, !d.FullExpand, d.Underapprox || d.FalseApprox); err != nil {
		return err
	}
	return d.original.streamReporter.Expand(edge)
}
