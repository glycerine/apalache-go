package duality

// Source: duality_rpfp.cpp:308-324,900-905.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (e *Edge) closeConstraints() error {
	m := e.Owner.context.manager
	for len(e.constraints) > 0 {
		n := len(e.constraints) - 1
		err := m.DelRef(&e.constraints[n])
		if e.constraints[n] == (ast.ASTRef{}) {
			e.constraints = e.constraints[:n]
		}
		if err != nil {
			return err
		}
	}
	if e.dual != (ast.ASTRef{}) {
		return m.DelRef(&e.dual)
	}
	return nil
}

// ToTermTree copies currently asserted dual formulas and additional constraints.
// Unasserted edges, open leaves and the skipped subtree contribute true.
func (g *RPFP) ToTermTree(root, skip *Node) (out *TermTree, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if root == nil || root.Owner != g || root.closed {
		return nil, ErrWrapperOwner
	}
	var children []*TermTree
	defer func() {
		if result != nil {
			for _, child := range children {
				result = errors.Join(result, child.Close())
			}
		}
	}()
	if root != skip && root.Outgoing != nil {
		for _, node := range root.Outgoing.Children {
			child, err := g.ToTermTree(node, skip)
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
	}
	edge := root.Outgoing
	var top *Expr
	var err error
	if root != skip && edge != nil && edge.dual != (ast.ASTRef{}) {
		h, e := g.context.manager.RefGet(edge.dual)
		if e != nil {
			return nil, e
		}
		top, err = NewExpr(g.context, h)
	} else {
		top, err = g.context.BoolVal(true)
	}
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, top.Close()) }()
	tree, err := NewTermTree(top, children)
	if err != nil {
		return nil, err
	}
	children = nil // Ownership transferred to tree.
	defer func() {
		if result != nil {
			result = errors.Join(result, tree.Close())
		}
	}()
	if root != skip && edge != nil {
		for _, ref := range edge.constraints {
			h, err := g.context.manager.RefGet(ref)
			if err != nil {
				return nil, err
			}
			term, err := NewExpr(g.context, h)
			if err != nil {
				return nil, err
			}
			err = errors.Join(tree.AddTerm(term), term.Close())
			if err != nil {
				return nil, err
			}
		}
	}
	return tree, nil
}

// AddUpperBound takes ownership of tree on success and copies the asserted
// node dual. The source uses true when no upper bound has been asserted.
func (g *RPFP) AddUpperBound(root *Node, tree *TermTree) (_ *TermTree, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if root == nil || root.Owner != g || root.closed {
		return nil, ErrWrapperOwner
	}
	if err := tree.open(); err != nil {
		return nil, err
	}
	var term *Expr
	var err error
	if root.dual == (ast.ASTRef{}) {
		term, err = g.context.BoolVal(true)
	} else {
		h, e := g.context.manager.RefGet(root.dual)
		if e != nil {
			return nil, e
		}
		term, err = NewExpr(g.context, h)
	}
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, term.Close()) }()
	return NewTermTree(term, []*TermTree{tree})
}
