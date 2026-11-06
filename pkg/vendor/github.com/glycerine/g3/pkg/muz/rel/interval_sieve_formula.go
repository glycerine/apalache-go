package rel

// Source: src/muz/rel/dl_sieve_relation.cpp sieve_relation::to_formula.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ToFormula substitutes each inner interval variable with its exact outer
// column; ignored columns consequently remain free.
func (r *IntervalSieve) ToFormula() (result ast.ASTRef, resultErr error) {
	if err := r.open(); err != nil {
		return result, err
	}
	innerSorts := make([]ast.ASTHandle, len(r.innerToSig))
	for inner, outer := range r.innerToSig {
		innerSorts[inner] = r.sorts[outer]
	}
	formula, err := r.inner.ToFormula(r.manager, innerSorts)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&formula)) }()
	handle, err := r.manager.RefGet(formula)
	if err != nil {
		return result, err
	}
	bindings := make([]ast.ASTHandle, len(r.innerToSig))
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&roots[i]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	for inner, outer := range r.innerToSig {
		variable, e := r.manager.MkVar(uint32(outer), r.sorts[outer])
		if e != nil {
			return result, e
		}
		root, e := r.manager.NewRef(variable)
		if e != nil {
			return result, e
		}
		roots = append(roots, root)
		bindings[inner] = variable
	}
	subst, err := rewriter.NewVarSubstitution(r.manager, false)
	if subst != nil {
		defer func() { resultErr = errors.Join(resultErr, subst.Close()) }()
	}
	if err != nil {
		return result, err
	}
	result, err = r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = subst.Substitute(handle, bindings, result); err != nil {
		return result, err
	}
	return result, nil
}
