package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear::get_model proof assembly.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// advanceQuantifiedLinearProofStep retires the previously published proof
// step before publishing the newly constructed one. A refused retirement
// adopts the new outputs behind the same retryable cleanup frontier.
func advanceQuantifiedLinearProofStep(manager *ast.ASTManager, previous, next *base.Rule, current, proof ast.ASTRef, cause error) (*base.Rule, ast.ASTRef, error) {
	var previousRules []*base.Rule
	if previous != nil {
		previousRules = append(previousRules, previous)
	}
	cause = disposeRuleOwners(manager, previousRules, cause)
	if current != (ast.ASTRef{}) {
		cause = disposeASTRefs(manager, []ast.ASTRef{current}, cause)
	}
	if cause == nil {
		return next, proof, nil
	}
	if next != nil {
		cause = disposeRuleOwners(manager, []*base.Rule{next}, cause)
	}
	if proof != (ast.ASTRef{}) {
		cause = disposeASTRefs(manager, []ast.ASTRef{proof}, cause)
	}
	return nil, ast.ASTRef{}, cause
}
