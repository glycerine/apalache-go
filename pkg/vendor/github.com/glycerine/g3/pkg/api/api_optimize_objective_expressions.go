package api

// Source: src/api/api_opt.cpp:375-393, Z3_optimize_get_objectives.

import "github.com/glycerine/g3/pkg/ast"

// ObjectiveExpressions returns independently owning source display terms.
// Maximization terms are negated and soft groups are penalty sums, exactly as
// opt::context::get_objective constructs the native AST vector.
func (o *APIOptimize) ObjectiveExpressions() ([]ast.ASTRef, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return o.command.OptimizationObjectives()
}
