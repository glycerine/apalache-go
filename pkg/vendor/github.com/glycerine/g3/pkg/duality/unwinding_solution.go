package duality

// Source: duality_solver.cpp:768-811.
import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"strconv"
)

func NodeMarker(node *Node, other *Node) (*Expr, error) {
	if node == nil || node.closed {
		return nil, ErrWrapperOwner
	}
	name := "@m_" + strconv.Itoa(node.Number)
	if other != nil {
		if other.closed || other.Owner.context != node.Owner.context {
			return nil, ErrContextMismatch
		}
		name += "_" + strconv.Itoa(other.Number)
	}
	return node.Owner.context.BoolConst(name)
}
func UnionAnnotations(destination *Transformer, source *Node, markers bool) (result error) {
	if source == nil || source.closed {
		return ErrWrapperOwner
	}
	if !markers {
		return destination.UnionWith(source.Annotation)
	}
	temporary, err := source.Annotation.Clone()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, temporary.Close()) }()
	marker, err := NodeMarker(source, nil)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, marker.Close()) }()
	formula, err := temporary.Formula()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	marked, err := formula.And(marker)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, marked.Close()) }()
	h, err := marked.Raw()
	if err != nil {
		return err
	}
	if err = temporary.context.manager.SetRef(temporary.formula, h); err != nil {
		return err
	}
	return destination.UnionWith(temporary)
}
func (u *Unwinding) GenNodeSolutionFromIndSet(node *Node, destination *Transformer, markers bool, simplifier *interp.Manager) (result error) {
	if u == nil || node == nil || node.Owner != u.source || node.closed || simplifier == nil {
		return ErrWrapperOwner
	}
	m, err := simplifier.ASTManager()
	if err != nil {
		return err
	}
	if m != u.graph.context.manager {
		return ErrContextMismatch
	}
	if err = destination.SetEmpty(); err != nil {
		return err
	}
	for _, instance := range u.covering.instances[node] {
		contains, err := u.covering.Contains(instance)
		if err != nil {
			return err
		}
		if contains {
			if err = UnionAnnotations(destination, instance, markers); err != nil {
				return err
			}
		}
	}
	h, err := m.RefGet(destination.formula)
	if err != nil {
		return err
	}
	simplified, err := simplifier.Simplify(h)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&simplified)) }()
	h, err = m.RefGet(simplified)
	if err != nil {
		return err
	}
	return m.SetRef(destination.formula, h)
}
func (u *Unwinding) NodeSolutionFromIndSetFull(node *Node) (bool, error) {
	if u == nil || node == nil || node.Owner != u.source || node.closed {
		return false, ErrWrapperOwner
	}
	for _, instance := range u.covering.instances[node] {
		contains, err := u.covering.Contains(instance)
		if err != nil {
			return false, err
		}
		if contains {
			full, err := instance.Annotation.IsFull()
			if err != nil {
				return false, err
			}
			if full {
				return true, nil
			}
		}
	}
	return false, nil
}
