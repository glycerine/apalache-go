package qe

// Source: src/qe/qe.cpp, quant_elim_new::bind_variables.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func bindExprQuantifierVariables(m *ast.ASTManager, variables []ast.ASTHandle, formula ast.ASTHandle) (result ast.ASTRef, err error) {
	if m == nil {
		return result, ast.ErrASTHandle
	}
	result, err = m.NewRef(formula)
	if err != nil {
		return result, err
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	var used, sorts []ast.ASTHandle
	var names []symbol.Symbol
	for _, v := range variables {
		contains, e := ast.NewContainsApp(m, v)
		if e != nil {
			return result, e
		}
		found, e := contains.Check(formula)
		e = errors.Join(e, contains.Close())
		if e != nil {
			return result, e
		}
		if !found {
			continue
		}
		sort, e := m.GetSort(v)
		if e != nil {
			return result, e
		}
		decl, e := m.GetDecl(v)
		if e != nil {
			return result, e
		}
		name, e := m.GetASTName(decl)
		if e != nil {
			return result, e
		}
		used = append(used, v)
		sorts = append(sorts, sort)
		names = append(names, name)
	}
	if len(used) == 0 {
		return result, nil
	}
	abstractor, err := ast.NewExprAbstractor(m)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, abstractor.Close()) }()
	if err = abstractor.Abstract(0, used, formula, result); err != nil {
		return result, err
	}
	body, err := m.RefGet(result)
	if err != nil {
		return result, err
	}
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: sorts, Names: names, Body: body, Weight: 1})
	if err != nil {
		return result, err
	}
	if err = m.SetRef(result, quantifier); err != nil {
		return result, err
	}
	return result, nil
}
