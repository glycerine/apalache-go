package aig

// Source: src/tactic/aig/aig.cpp:776-789,944-1009,1530-1537.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type formulaConverter struct {
	manager *Manager
	cache   map[*node]ast.ASTRef
}

func (c *formulaConverter) base(n *node) (ast.ASTHandle, error) {
	if !n.and {
		return c.manager.ast.RefGet(n.variable)
	}
	ref, ok := c.cache[n]
	if !ok {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return c.manager.ast.RefGet(ref)
}

func (c *formulaConverter) invert(h ast.ASTHandle) (ast.ASTHandle, error) {
	m := c.manager.ast
	isNot, child, err := m.IsNot(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if isNot {
		return child, nil
	}
	if m.IsTrue(h) {
		return m.MkFalse()
	}
	return m.MkNot(h)
}

func (c *formulaConverter) build(n *node) (result ast.ASTRef, err error) {
	m := c.manager
	var temporary [3]ast.ASTRef
	defer func() {
		for i := len(temporary) - 1; i >= 0; i-- {
			err = errors.Join(err, m.releaseAST(&temporary[i]))
		}
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.releaseAST(&result))
			result = ast.ASTRef{}
		}
	}()
	var negated [2]ast.ASTHandle
	for i, child := range n.children {
		base, e := c.base(child.node)
		if e != nil {
			return ast.ASTRef{}, e
		}
		if !child.inverted {
			base, e = c.invert(base)
			if e != nil {
				return ast.ASTRef{}, e
			}
		}
		temporary[i], e = m.ast.NewRef(base)
		if e != nil {
			return ast.ASTRef{}, e
		}
		negated[i] = base
	}
	or, e := m.ast.MkOr(negated[0], negated[1])
	if e != nil {
		return ast.ASTRef{}, e
	}
	temporary[2], e = m.ast.NewRef(or)
	if e != nil {
		return ast.ASTRef{}, e
	}
	and, e := m.ast.MkNot(or)
	if e != nil {
		return ast.ASTRef{}, e
	}
	return m.ast.NewRef(and)
}

func (c *formulaConverter) cleanup() error {
	var err error
	for _, ref := range c.cache {
		err = errors.Join(err, c.manager.releaseAST(&ref))
	}
	c.cache = nil
	return err
}

// ToFormula follows the source's iterative naive AIG-to-expression path.
// The returned ASTRef owns the result; the caller closes it with ASTManager.DelRef.
func (m *Manager) ToFormula(r *Ref) (out ast.ASTRef, err error) {
	l, e := r.literal(m)
	if e != nil {
		return ast.ASTRef{}, e
	}
	c := formulaConverter{manager: m, cache: make(map[*node]ast.ASTRef)}
	defer func() {
		err = errors.Join(err, c.cleanup())
		if err != nil && out != (ast.ASTRef{}) {
			err = errors.Join(err, m.releaseAST(&out))
			out = ast.ASTRef{}
		}
	}()
	todo := []*node{l.node}
	for len(todo) != 0 {
		last := len(todo) - 1
		n := todo[last]
		if !n.and {
			todo = todo[:last]
			continue
		}
		if _, ok := c.cache[n]; ok {
			todo = todo[:last]
			continue
		}
		ready := true
		for _, child := range n.children {
			if child.node.and {
				if _, ok := c.cache[child.node]; !ok {
					todo = append(todo, child.node)
					ready = false
				}
			}
		}
		if !ready {
			continue
		}
		owned, e := c.build(n)
		if e != nil {
			return ast.ASTRef{}, e
		}
		c.cache[n] = owned
		todo = todo[:last]
	}
	base, e := c.base(l.node)
	if e != nil {
		return ast.ASTRef{}, e
	}
	if l.inverted {
		base, e = c.invert(base)
		if e != nil {
			return ast.ASTRef{}, e
		}
	}
	out, e = m.ast.NewRef(base)
	return out, e
}
