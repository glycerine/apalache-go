package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (r *bvArrayRewriter) forall(sort, body ast.ASTHandle) (ast.ASTHandle, error) {
	name, e := r.owner.manager.Symbols.Intern("x")
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return r.owner.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{name}, Body: body})
}
func (r *bvArrayRewriter) apply(f, index ast.ASTHandle) (ast.ASTHandle, error) {
	return r.owner.manager.MkApp(f, []ast.ASTHandle{index})
}
func (r *bvArrayRewriter) extensionalEquality(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m := r.owner.manager
	fa, e := r.functionFor(a)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	fb, e := r.functionFor(b)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	s, e := m.GetSort(a)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	index, e := r.indexSort(s)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	x, e := m.MkVar(0, index)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	av, e := r.apply(fa, x)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	bv, e := r.apply(fb, x)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	eq, e := m.MkEq(av, bv)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return r.forall(index, eq)
}
func (r *bvArrayRewriter) addDefinition(array ast.ASTHandle, value func(ast.ASTHandle) (ast.ASTHandle, error)) (ast.ASTHandle, error) {
	m := r.owner.manager
	f, e := r.functionFor(array)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	s, e := m.GetSort(array)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	index, e := r.indexSort(s)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	x, e := m.MkVar(0, index)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	left, e := r.apply(f, x)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	right, e := value(x)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	eq, e := m.MkEq(left, right)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	q, e := r.forall(index, eq)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if e = r.addExtra(q); e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkAsArray(f)
}
