package core

// Source: src/tactic/core/injectivity_tactic.cpp InjHelper.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type injectivityRelations struct {
	self       *injectivityRelations
	manager    *ast.ASTManager
	values     map[ast.ASTHandle]map[ast.ASTHandle]bool
	order      []ast.ASTHandle
	owned      []ast.ASTHandle
	ownerBytes uint64
	released   int
	closed     bool
}

func newInjectivityRelations(m *ast.ASTManager) (*injectivityRelations, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(injectivityRelations{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	r := &injectivityRelations{manager: m, values: make(map[ast.ASTHandle]map[ast.ASTHandle]bool), ownerBytes: n}
	r.self = r
	return r, nil
}
func (r *injectivityRelations) open() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return arena.ErrClosed
	}
	return nil
}
func (r *injectivityRelations) Insert(f, g ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	set, found := r.values[f]
	if !found {
		if err := r.manager.IncRef(f); err != nil {
			return err
		}
		r.owned = append(r.owned, f)
		r.order = append(r.order, f)
		set = make(map[ast.ASTHandle]bool)
		r.values[f] = set
	}
	if set[g] {
		return nil
	}
	if err := r.manager.IncRef(g); err != nil {
		return err
	}
	r.owned = append(r.owned, g)
	set[g] = true
	return nil
}
func (r *injectivityRelations) Contains(f ast.ASTHandle) (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	_, found := r.values[f]
	return found, nil
}
func (r *injectivityRelations) Find(f, g ast.ASTHandle) (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	return r.values[f][g], nil
}
func (r *injectivityRelations) Close() (err error) {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	for r.released < len(r.owned) {
		i := len(r.owned) - 1 - r.released
		consumed, e := r.manager.DecRefConsumed(r.owned[i])
		if consumed {
			r.released++
		}
		if e != nil {
			return errors.Join(err, e)
		}
	}
	r.manager.Budget().Release(r.ownerBytes)
	r.ownerBytes = 0
	r.closed = true
	return err
}
