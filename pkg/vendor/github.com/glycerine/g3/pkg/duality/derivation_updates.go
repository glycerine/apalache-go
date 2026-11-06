package duality

// Source: duality_solver.cpp:2477-2487,2514-2552; Update:1561-1575.
import (
	"errors"
	"github.com/glycerine/g3/pkg/solver"
)

func (d *BacktrackingDerivation) AtCurrentStackLevel(node *Node) bool {
	if d == nil || len(d.stack) == 0 {
		return false
	}
	for _, expanded := range d.stack[len(d.stack)-1].expansions {
		if expanded == node {
			return true
		}
	}
	return false
}
func replaceNodeAnnotation(node *Node, annotation *Transformer) error {
	copy, err := annotation.Clone()
	if err != nil {
		return err
	}
	if err = node.Annotation.Close(); err != nil {
		return errors.Join(err, copy.Close())
	}
	node.Annotation = copy
	return nil
}

// RecordAnnotationUpdate performs the source fact intersection and queues
// affected instances. A bound source covering is invalidated before the
// original fact changes. Learned facts reach the owning search reporter.
func (d *BacktrackingDerivation) RecordAnnotationUpdate(node *Node, aux solver.Solver) (bool, error) {
	if d == nil || node == nil || node.Owner != d.tree || node.Map == nil {
		return false, ErrWrapperOwner
	}
	original := node.Map
	redundant, err := original.Annotation.SubsetEq(node.Annotation, aux)
	if err != nil {
		return false, err
	}
	if redundant {
		return false, nil
	}
	if err := d.original.streamReporter.Update(original, node.Annotation, false); err != nil {
		return false, err
	}
	if err := d.original.conjectureReporter.Update(original, node.Annotation); err != nil {
		return false, err
	}
	if d.sourceCovering != nil {
		if err := d.sourceCovering.Update(original); err != nil {
			return false, err
		}
	}
	if err = original.Annotation.IntersectWith(node.Annotation); err != nil {
		return false, err
	}
	instances := append([]*Node(nil), d.nodeMap[original]...)
	for _, other := range instances {
		if other == node || !(len(node.Incoming) > 0 && len(other.Incoming) > 0 && d.AtCurrentStackLevel(other.Incoming[0].Parent)) {
			d.updatedNodes = append(d.updatedNodes, other)
			if other != node {
				if err = replaceNodeAnnotation(other, node.Annotation); err != nil {
					return false, err
				}
			}
		}
	}
	return true, nil
}
func (d *BacktrackingDerivation) RemoveUpdateNodesAtCurrentLevel() error {
	if d == nil {
		return ErrWrapperOwner
	}
	kept := d.updatedNodes[:0]
	for _, node := range d.updatedNodes {
		if len(node.Incoming) == 0 {
			return ErrWrapperOwner
		}
		if !d.AtCurrentStackLevel(node.Incoming[0].Parent) {
			kept = append(kept, node)
		}
	}
	d.updatedNodes = kept
	return nil
}
func (d *BacktrackingDerivation) HandleUpdatedNodes() error {
	if d == nil {
		return ErrWrapperOwner
	}
	for n := 0; n < len(d.updatedNodes); {
		node := d.updatedNodes[n]
		if node.Map == nil || len(node.Incoming) == 0 {
			return ErrWrapperOwner
		}
		if err := replaceNodeAnnotation(node, node.Map.Annotation); err != nil {
			return err
		}
		if err := d.tree.ConstrainParent(node.Incoming[0], node); err != nil {
			return err
		}
		if d.AtCurrentStackLevel(node.Incoming[0].Parent) {
			d.updatedNodes = append(d.updatedNodes[:n], d.updatedNodes[n+1:]...)
		} else {
			n++
		}
	}
	return nil
}

// BindSourceCovering connects the source unwinding covering relation used by
// outer Update; it belongs to the original graph, not the derivation graph.
func (d *BacktrackingDerivation) BindSourceCovering(covering *Covering) error {
	if d == nil || covering == nil || covering.graph != d.original {
		return ErrWrapperOwner
	}
	d.sourceCovering = covering
	return nil
}
