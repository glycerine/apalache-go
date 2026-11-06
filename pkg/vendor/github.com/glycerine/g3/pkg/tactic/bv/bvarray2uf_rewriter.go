package bv

// Source: src/tactic/bv/bvarray2uf_rewriter.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type bvArrayRewriter struct {
	owner              *BVArray2UFTactic
	arrays             map[ast.ASTHandle]ast.ASTHandle
	keys, declarations []ast.ASTHandle
	extra              []ast.ASTHandle
	extension          *tactic.ExtensionModelConverter
	filter             *tactic.FilterModelConverter
}

func newBVArrayRewriter(t *BVArray2UFTactic) *bvArrayRewriter {
	return &bvArrayRewriter{owner: t, arrays: make(map[ast.ASTHandle]ast.ASTHandle)}
}
func (r *bvArrayRewriter) close() (err error) {
	m := r.owner.manager
	for i := len(r.extra) - 1; i >= 0; i-- {
		err = errors.Join(err, m.DecRef(r.extra[i]))
	}
	for i := len(r.declarations) - 1; i >= 0; i-- {
		err = errors.Join(err, m.DecRef(r.declarations[i]))
		err = errors.Join(err, m.DecRef(r.keys[i]))
	}
	if err == nil {
		r.extra = nil
		r.declarations = nil
		r.keys = nil
		r.arrays = nil
	}
	return err
}
func (r *bvArrayRewriter) addExtra(h ast.ASTHandle) error {
	if e := r.owner.manager.IncRef(h); e != nil {
		return e
	}
	r.extra = append(r.extra, h)
	return nil
}

func (r *bvArrayRewriter) rewrite(root ast.ASTHandle) (ast.ASTHandle, error) {
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	var visit func(ast.ASTHandle) (ast.ASTHandle, error)
	visit = func(h ast.ASTHandle) (ast.ASTHandle, error) {
		if v, ok := memo[h]; ok {
			return v, nil
		}
		m := r.owner.manager
		k, e := m.GetKind(h)
		if e != nil {
			return h, e
		}
		if k == ast.ASTQuantifier {
			return h, tactic.NewTacticException("bvarray2uf: quantified bit-vector arrays are not implemented")
		}
		if k != ast.ASTApp {
			memo[h] = h
			return h, nil
		}
		n, e := m.GetNumArgs(h)
		if e != nil {
			return h, e
		}
		args := make([]ast.ASTHandle, n)
		changed := false
		for i := uint32(0); i < n; i++ {
			a, x := m.GetArg(h, i)
			if x != nil {
				return h, x
			}
			args[i], x = visit(a)
			if x != nil {
				return h, x
			}
			changed = changed || args[i] != a
		}
		current := h
		if changed {
			d, x := m.GetDecl(h)
			if x != nil {
				return h, x
			}
			current, x = m.MkApp(d, args)
			if x != nil {
				return h, x
			}
		}
		result, handled, x := r.reduce(current, args)
		if x != nil {
			return h, x
		}
		if handled {
			current = result
		}
		memo[h] = current
		return current, nil
	}
	return visit(root)
}
