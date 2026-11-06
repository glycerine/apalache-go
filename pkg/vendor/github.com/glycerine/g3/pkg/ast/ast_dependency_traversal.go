package ast

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (d *astDependencyManager) unmarkTodo() error {
	var first error
	for _, r := range d.todo {
		c, e := d.node(r)
		if e == nil {
			c.marked = false
			e = d.store(r, c)
		}
		if first == nil {
			first = e
		}
	}
	d.todo = d.todo[:0]
	return first
}
func (d *astDependencyManager) traverse(root arena.SmallBytesRef, target astNodeRef, out *[]ASTHandle) (bool, error) {
	if d.pending {
		return false, ErrASTPending
	}
	if dependencyNull(root) {
		return false, nil
	}
	d.todo = d.todo[:0]
	c, e := d.node(root)
	if e != nil {
		return false, e
	}
	c.marked = true
	if e = d.store(root, c); e != nil {
		return false, e
	}
	d.todo = append(d.todo, root)
	// Clear every visited dependency mark on success and checked traversal errors.
	defer d.unmarkTodo()
	for q := 0; q < len(d.todo); q++ {
		c, e := d.node(d.todo[q])
		if e != nil {
			return false, e
		}
		if c.leaf {
			if out != nil {
				*out = append(*out, d.owner.external(c.value))
			} else if c.value == target {
				return true, nil
			}
		} else {
			for _, child := range [2]arena.SmallBytesRef{c.left, c.right} {
				v, e := d.node(child)
				if e != nil {
					return false, e
				}
				if !v.marked {
					d.todo = append(d.todo, child)
					v.marked = true
					if e = d.store(child, v); e != nil {
						return false, e
					}
				}
			}
		}
	}
	return false, nil
}
func (m *ASTManager) DependencyContains(d ASTDependency, h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.dependencies.valid(d)
	if e != nil {
		return false, e
	}
	var v astNodeRef
	if !h.IsNull() {
		v, e = m.requireExpr(h)
		if e != nil {
			return false, e
		}
	}
	return m.dependencies.traverse(r, v, nil)
}

// DependencyLinearizeRaw appends leaf expressions in traversal order. The caller
// owns the slice; its handles borrow the dependency's expression references.
// On error, the returned slice contains every completed output append.
func (m *ASTManager) DependencyLinearizeRaw(d ASTDependency, out []ASTHandle) ([]ASTHandle, error) {
	if e := m.lock(); e != nil {
		return out, e
	}
	defer m.mu.Unlock()
	r, e := m.dependencies.valid(d)
	if e != nil {
		return out, e
	}
	_, e = m.dependencies.traverse(r, astNodeRef{}, &out)
	return out, e
}

// DependencyLinearize appends reachable leaves and removes duplicates from the
// entire output, retaining each expression's first occurrence. It owns no AST
// references and does not modify the expressions' shared mark bits.
func (m *ASTManager) DependencyLinearize(d ASTDependency, out []ASTHandle) ([]ASTHandle, error) {
	if e := m.lock(); e != nil {
		return out, e
	}
	defer m.mu.Unlock()
	r, e := m.dependencies.valid(d)
	if e != nil {
		return out, e
	}
	if _, e = m.dependencies.traverse(r, astNodeRef{}, &out); e != nil {
		return out, e
	}
	seen := make(map[ASTHandle]struct{}, len(out))
	j := 0
	for _, h := range out {
		if _, e = m.requireExpr(h); e != nil {
			return out, e
		}
		if _, found := seen[h]; !found {
			seen[h] = struct{}{}
			out[j] = h
			j++
		}
	}
	return out[:j], nil
}
