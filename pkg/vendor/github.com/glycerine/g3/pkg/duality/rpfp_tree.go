package duality

// Source: duality_rpfp.cpp:325-345,851-860.
import "errors"

func (g *RPFP) ToGoalTree(root *Node) (out *TermTree, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if root == nil || root.Owner != g || root.closed {
		return nil, ErrWrapperOwner
	}
	b, vars, err := g.RedVars(root)
	if err != nil {
		return nil, err
	}
	defer func() {
		result = errors.Join(result, b.Close())
		for _, v := range vars {
			result = errors.Join(result, v.Close())
		}
	}()
	h, err := g.context.manager.RefGet(root.Name)
	if err != nil {
		return nil, err
	}
	decl, err := NewFuncDecl(g.context, h)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, decl.Close()) }()
	goal, err := decl.Call(vars...)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, goal.Close()) }()
	var children []*TermTree
	defer func() {
		if result != nil {
			for _, child := range children {
				result = errors.Join(result, child.Close())
			}
		}
	}()
	if root.Outgoing != nil {
		for _, node := range root.Outgoing.Children {
			child, err := g.ToGoalTree(node)
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
	}
	return NewTermTree(goal, children)
}
func (g *RPFP) GetGoalTree(root *Node) (_ *TermTree, result error) {
	child, err := g.ToGoalTree(root)
	if err != nil {
		return nil, err
	}
	falsity, err := g.context.BoolVal(false)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	defer func() { result = errors.Join(result, falsity.Close()) }()
	tree, err := NewTermTree(falsity, []*TermTree{child})
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return tree, nil
}

// DecodeTree applies children before their parent, as the source does.
func (g *RPFP) DecodeTree(root *Node, interpolants *TermTree) error {
	if err := g.open(); err != nil {
		return err
	}
	if root == nil || root.Owner != g || root.closed {
		return ErrWrapperOwner
	}
	if err := interpolants.open(); err != nil {
		return err
	}
	if len(interpolants.children) > 0 {
		if root.Outgoing == nil || len(interpolants.children) < len(root.Outgoing.Children) {
			return ErrWrapperOwner
		}
		for n, child := range root.Outgoing.Children {
			if err := g.DecodeTree(child, interpolants.children[n]); err != nil {
				return err
			}
		}
	}
	return g.SetAnnotation(root, interpolants.term)
}
