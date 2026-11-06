package duality

// Source: src/muz/duality/duality_dl_interface.cpp, construction of the
// universally closed query-implies-false clause.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// QueryClause returns an owning reference to the source Duality query clause.
// Missing free-variable sorts use Bool, and bound names are numeric indices in
// reverse de Bruijn order.
func QueryClause(m *ast.ASTManager, query ast.ASTHandle) (ast.ASTRef, error) {
	if m == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	used, err := ast.NewASTUsedVars(m)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = used.Process(query); err != nil {
		return ast.ASTRef{}, err
	}
	count := used.MaxFoundVarIndexPlusOne()
	sorts := make([]ast.ASTHandle, 0, count)
	names := make([]symbol.Symbol, 0, count)
	for i := count; i > 0; i-- {
		index := i - 1
		sort := used.Contains(index)
		if sort.IsNull() {
			sort, err = m.MkBoolSort()
			if err != nil {
				return ast.ASTRef{}, err
			}
		}
		sorts = append(sorts, sort)
		names = append(names, symbol.Numerical(index))
	}
	falsity, err := m.MkFalse()
	if err != nil {
		return ast.ASTRef{}, err
	}
	implication, err := m.MkImplies(query, falsity)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if count == 0 {
		return m.NewRef(implication)
	}
	ref, err := m.NewRef(implication)
	if err != nil {
		return ast.ASTRef{}, err
	}
	quantified, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: implication})
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&ref))
	}
	result, err := m.NewRef(quantified)
	if closeErr := m.DelRef(&ref); closeErr != nil {
		if result != (ast.ASTRef{}) {
			closeErr = errors.Join(closeErr, m.DelRef(&result))
		}
		return ast.ASTRef{}, errors.Join(err, closeErr)
	}
	return result, err
}
