package core

// Source: src/tactic/core/dom_simplify_tactic.cpp:25-161 and
// dom_simplify_tactic.h:29-61.

import "github.com/glycerine/g3/pkg/ast"

type expressionDominators struct {
	manager   *ast.ASTManager
	root      ast.ASTHandle
	post      []ast.ASTHandle
	postIndex map[ast.ASTHandle]uint32
	parents   map[ast.ASTHandle][]ast.ASTHandle
	idom      map[ast.ASTHandle]ast.ASTHandle
	tree      map[ast.ASTHandle][]ast.ASTHandle
}

func newExpressionDominators(m *ast.ASTManager) *expressionDominators {
	return &expressionDominators{manager: m}
}
func (d *expressionDominators) reset() {
	d.root = ast.ASTHandle{}
	d.post = nil
	d.postIndex = make(map[ast.ASTHandle]uint32)
	d.parents = make(map[ast.ASTHandle][]ast.ASTHandle)
	d.idom = make(map[ast.ASTHandle]ast.ASTHandle)
	d.tree = make(map[ast.ASTHandle][]ast.ASTHandle)
}

func (d *expressionDominators) compile(root ast.ASTHandle) (bool, error) {
	d.reset()
	d.root = root
	if err := d.computePostOrder(root); err != nil {
		return false, err
	}
	ok, err := d.computeDominators()
	if err != nil || !ok {
		return ok, err
	}
	// C++ inserts the root into m_doms first and then inserts other nodes in
	// post-order. Preserve that order when extracting the immediate-dominator
	// tree; ranging over a Go map makes traversal order unstable.
	if parent, ok := d.idom[d.root]; ok {
		d.tree[parent] = append(d.tree[parent], d.root)
	}
	for _, child := range d.post {
		if child == d.root {
			continue
		}
		if parent, ok := d.idom[child]; ok {
			d.tree[parent] = append(d.tree[parent], child)
		}
	}
	return true, nil
}

func (d *expressionDominators) computePostOrder(root ast.ASTHandle) error {
	type frame struct {
		h    ast.ASTHandle
		next uint32
		args uint32
	}
	marked := make(map[ast.ASTHandle]bool)
	stack := []frame{{h: root}}
	for len(stack) > 0 {
		i := len(stack) - 1
		f := &stack[i]
		if marked[f.h] {
			stack = stack[:i]
			continue
		}
		kind, err := d.manager.GetKind(f.h)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			marked[f.h] = true
			stack = stack[:i]
			continue
		}
		if f.args == 0 {
			f.args, err = d.manager.GetNumArgs(f.h)
			if err != nil {
				return err
			}
		}
		pushed := false
		for f.next < f.args {
			arg, e := d.manager.GetArg(f.h, f.next)
			if e != nil {
				return e
			}
			f.next++
			if !marked[arg] {
				stack = append(stack, frame{h: arg})
				pushed = true
				break
			}
		}
		if pushed {
			continue
		}
		marked[f.h] = true
		d.postIndex[f.h] = uint32(len(d.post))
		d.post = append(d.post, f.h)
		stack = stack[:i]
		for j := uint32(0); j < f.args; j++ {
			arg, e := d.manager.GetArg(f.h, j)
			if e != nil {
				return e
			}
			d.parents[arg] = append(d.parents[arg], f.h)
		}
	}
	return nil
}

func (d *expressionDominators) intersect(x, y ast.ASTHandle) (ast.ASTHandle, error) {
	for x != y {
		nx, xok := d.postIndex[x]
		ny, yok := d.postIndex[y]
		if !xok || !yok {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		if nx < ny {
			x = d.idom[x]
		} else {
			y = d.idom[y]
		}
	}
	return x, nil
}

func (d *expressionDominators) computeDominators() (bool, error) {
	if d.root.IsNull() {
		return false, ast.ErrASTHandle
	}
	d.idom[d.root] = d.root
	iterations := uint32(1)
	for {
		changed := false
		if len(d.post) > 0 && d.post[len(d.post)-1] != d.root {
			return false, ast.ErrASTDomain
		}
		for i := 0; i+1 < len(d.post); i++ {
			child := d.post[i]
			parents := d.parents[child]
			var next ast.ASTHandle
			for _, p := range parents {
				if _, known := d.idom[p]; !known {
					continue
				}
				if next.IsNull() {
					next = p
				} else {
					var err error
					next, err = d.intersect(next, p)
					if err != nil {
						return false, err
					}
				}
			}
			if next.IsNull() {
				if len(parents) == 0 {
					continue
				}
				next = parents[0]
			}
			if old, ok := d.idom[child]; !ok || old != next {
				d.idom[child] = next
				changed = true
			}
		}
		iterations *= 2
		if !changed {
			return true, nil
		}
		if iterations > uint32(len(d.post)) {
			return false, nil
		}
	}
}
