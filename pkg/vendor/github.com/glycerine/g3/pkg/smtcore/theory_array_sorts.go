package smtcore

// Source: src/smt/theory_array_base.cpp register_sort, restore_sorts,
// push_scope_eh/pop_scope_eh and source sort2skolem destruction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTArrayTheory) registerArraySort(sort ast.ASTHandle) (err error) {
	if _, found := t.sortDeclarations[sort]; found {
		return nil
	}
	m := t.state.manager
	dimension, err := m.ArrayArity(sort)
	if err != nil {
		return err
	}
	var declarations []ast.ASTHandle
	defer func() {
		if err != nil {
			for _, declaration := range declarations {
				err = errors.Join(err, m.DecRef(declaration))
			}
		}
	}()
	for i := uint32(0); i < dimension; i++ {
		declaration, e := m.MkArrayExtDecl(sort, i)
		if e != nil {
			return e
		}
		if e = m.IncRef(declaration); e != nil {
			return e
		}
		declarations = append(declarations, declaration)
	}
	t.sortDeclarations[sort] = declarations
	t.sortTrail = append(t.sortTrail, sort)
	return nil
}

func (t *SMTArrayTheory) restoreArraySorts(limit int) error {
	if limit < 0 || limit > len(t.sortTrail) {
		return ast.ErrASTDomain
	}
	m := t.state.manager
	for len(t.sortTrail) > limit {
		sort := t.sortTrail[len(t.sortTrail)-1]
		declarations := t.sortDeclarations[sort]
		for len(declarations) != 0 {
			if err := m.DecRef(declarations[len(declarations)-1]); err != nil {
				return err
			}
			declarations = declarations[:len(declarations)-1]
			t.sortDeclarations[sort] = declarations
		}
		delete(t.sortDeclarations, sort)
		t.sortTrail = t.sortTrail[:len(t.sortTrail)-1]
	}
	return nil
}

func (t *SMTArrayTheory) Close() error {
	if t == nil {
		return ast.ErrASTHandle
	}
	if len(t.sortTrail) == 0 && len(t.epsilonTrail) == 0 {
		return nil
	}
	if _, err := t.State(); err != nil {
		return err
	}
	if err := t.restoreArrayEpsilons(0); err != nil {
		return err
	}
	return t.restoreArraySorts(0)
}
