package fp

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Query mirrors rel_context's result status for an imprecise relation plugin.
// A query with no requested predicates is true and all-empty relations are false, but a
// nonempty interval abstraction is only an over-approximate witness and must
// therefore remain unknown.
func (e *IntervalDatalogEvaluator) Query(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	if len(declarations) == 0 {
		return lbool.True, nil
	}
	for _, declaration := range declarations {
		if e.relations[declaration] == nil {
			return lbool.Undef, fmt.Errorf("%w: undeclared query predicate", ErrIntervalDatalogFragment)
		}
	}
	for _, declaration := range declarations {
		relation := e.relations[declaration]
		empty, err := e.empty(relation, e.sorts[declaration])
		if err != nil {
			return lbool.Undef, err
		}
		if !empty {
			return lbool.Undef, nil
		}
	}
	return lbool.False, nil
}

// RelationFormula returns a caller-owned AST representation of the current
// abstract relation for certificate/model presentation.
func (e *IntervalDatalogEvaluator) RelationFormula(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	relation := e.relations[declaration]
	if relation == nil {
		return ast.ASTRef{}, fmt.Errorf("%w: undeclared formula predicate", ErrIntervalDatalogFragment)
	}
	return relation.ToFormula(e.manager, e.sorts[declaration])
}
