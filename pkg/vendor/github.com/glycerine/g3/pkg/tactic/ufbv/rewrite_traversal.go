package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:319-354;
// src/util/ref_buffer.h:35-51 and src/util/array_map.h:62-91.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// rewriteTraversal is the concrete UFBV child-visit frame. Its todo slice
// owns one AST reference per entry; the cache owns its own entries separately.
type rewriteTraversal struct {
	manager         *ast.ASTManager
	trace           *diagnostic.TraceTags
	cache           *rewriteCache
	todo            []ast.ASTHandle
	pending         []ast.ASTHandle
	newExprs        []ast.ASTRef
	newExprsDrained int
	closed          bool
}

func newRewriteTraversal(m *ast.ASTManager) (*rewriteTraversal, error) {
	c, e := newRewriteCache(m)
	if e != nil {
		return nil, e
	}
	return &rewriteTraversal{manager: m, cache: c}, nil
}

func (r *rewriteTraversal) check() error {
	if r == nil || r.manager == nil || r.cache == nil || r.closed {
		return ast.ErrASTHandle
	}
	return r.cache.check()
}

func (r *rewriteTraversal) pushTodo(h ast.ASTHandle) error {
	if e := r.check(); e != nil {
		return e
	}
	if e := r.manager.IncRef(h); e != nil {
		return e
	}
	r.todo = append(r.todo, h)
	return nil
}

func (r *rewriteTraversal) popTodo() error {
	if e := r.check(); e != nil {
		return e
	}
	if len(r.todo) == 0 {
		return ast.ErrASTDomain
	}
	last := len(r.todo) - 1
	h := r.todo[last]
	r.todo[last] = ast.ASTHandle{}
	r.todo = r.todo[:last]
	consumed, e := r.manager.DecRefConsumed(h)
	if !consumed {
		r.pending = append(r.pending, h)
		if e == nil {
			return ast.ErrASTPending
		}
	}
	return e
}

// visitChildren scans an application's arguments in reverse. A child already
// on the todo stack is marked complete in the cache to break the source cycle.
func (r *rewriteTraversal) visitChildren(app ast.ASTHandle) (bool, error) {
	if e := r.check(); e != nil {
		return false, e
	}
	count, e := r.manager.GetNumArgs(app)
	if e != nil {
		return false, e
	}
	allReady := true
	for j := count; j > 0; j-- {
		child, e := r.manager.GetArg(app, j-1)
		if e != nil {
			return false, e
		}
		present, e := r.cache.contains(child)
		if e != nil {
			return false, e
		}
		if present {
			cached, _, e := r.cache.get(child)
			if e != nil {
				return false, e
			}
			if cached.done {
				continue
			}
		}
		recursive := false
		for i := len(r.todo); i > 0; i-- {
			if r.todo[i-1] == child {
				recursive = true
				break
			}
		}
		if recursive {
			if e := r.traceCycle(app, child); e != nil {
				return false, e
			}
			if e := r.cache.insert(child, child, true); e != nil {
				return false, e
			}
			continue
		}
		if e := r.pushTodo(child); e != nil {
			return false, e
		}
		allReady = false
	}
	return allReady, nil
}

func (r *rewriteTraversal) close() error {
	if r == nil || r.closed {
		return nil
	}
	// m_new_exprs is declared after the cache and todo buffers in the source.
	// Its owned expressions therefore retire first, in insertion order.
	for r.newExprsDrained < len(r.newExprs) {
		ref := &r.newExprs[r.newExprsDrained]
		e := r.manager.DelRef(ref)
		if *ref == (ast.ASTRef{}) {
			r.newExprsDrained++
		}
		if e != nil {
			return e
		}
	}
	r.newExprs = nil
	r.newExprsDrained = 0
	if e := r.cache.close(); e != nil {
		return e
	}
	for len(r.todo) != 0 {
		h := r.todo[0]
		consumed, e := r.manager.DecRefConsumed(h)
		if consumed {
			r.todo[0] = ast.ASTHandle{}
			r.todo = r.todo[1:]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	for len(r.pending) != 0 {
		last := len(r.pending) - 1
		consumed, e := r.manager.DecRefConsumed(r.pending[last])
		if consumed {
			r.pending[last] = ast.ASTHandle{}
			r.pending = r.pending[:last]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	r.closed = true
	return nil
}
