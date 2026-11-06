package rel

// Source: src/muz/rel/udoc_relation.cpp udoc_relation::to_formula.

import "github.com/glycerine/g3/pkg/ast"

// ToFormula returns the source-style disjunction of the relation's documents.
// The returned AST reference is owned by the caller.
func (r *UDocRelation) ToFormula(manager *ast.ASTManager) (ast.ASTRef, error) {
	if err := r.check(); err != nil {
		return ast.ASTRef{}, err
	}
	if manager == nil {
		return ast.ASTRef{}, ErrDOC
	}
	disjuncts := make([]ast.ASTHandle, 0, len(r.elems.values))
	for _, document := range r.elems.values {
		formula, err := r.manager.ToFormula(manager, document)
		if err != nil {
			return ast.ASTRef{}, err
		}
		disjuncts = append(disjuncts, formula)
	}
	formula, err := manager.MkOrArray(disjuncts)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return manager.NewRef(formula)
}
