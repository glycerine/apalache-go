package qe

// Source: src/qe/qe_array_plugin.cpp, solve_select.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// TrySelectEquality solves select(A, indices...) = rhs using a fresh array.
// It publishes the rewritten formula and replaces A in the context variable list.
func (p *ArrayPlugin) TrySelectEquality(lhs, rhs, formula ast.ASTHandle) (solved bool, err error) {
	idx, levels, ok, e := p.arrayChain(lhs, rhs, ast.ASTArrayOpSelect)
	if e != nil || !ok {
		return false, e
	}
	if len(levels) != 1 {
		return false, nil
	}
	variable, e := p.context.Variable(idx)
	if e != nil {
		return false, e
	}
	sort, e := p.manager.GetSort(variable)
	if e != nil {
		return false, e
	}
	fresh, e := p.manager.MkFreshConst("B", sort)
	if e != nil {
		return false, e
	}
	freshRef, e := p.manager.NewRef(fresh)
	if e != nil {
		return false, e
	}
	defer func() {
		if freshRef != (ast.ASTRef{}) {
			err = errors.Join(err, p.manager.DelRef(&freshRef))
		}
	}()
	args := make([]ast.ASTHandle, 0, len(levels[0])+2)
	args = append(args, fresh)
	args = append(args, levels[0]...)
	args = append(args, rhs)
	store, e := p.manager.MkFamilyApp(p.family, ast.ASTArrayOpStore, nil, args)
	if e != nil {
		return false, e
	}
	storeRef, e := p.manager.NewRef(store)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, p.manager.DelRef(&storeRef)) }()
	result, e := p.manager.NewRef(formula)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, p.manager.DelRef(&result)) }()
	rw, e := rewriter.NewExprSafeReplace(p.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, rw.Close()) }()
	if e = rw.ApplySubstitution(variable, store, result); e != nil {
		return false, e
	}
	rewritten, e := p.manager.RefGet(result)
	if e != nil {
		return false, e
	}
	if e = p.context.adoptVariable(fresh, &freshRef); e != nil {
		return false, e
	}
	if e = p.context.EliminateVariable(idx, rewritten, store); e != nil {
		return false, e
	}
	return true, nil
}
