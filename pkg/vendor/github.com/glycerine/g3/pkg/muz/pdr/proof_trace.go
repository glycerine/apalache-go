package pdr

// Source: model_search::get_proof_trace, context::get_proof, and
// context::mk_sat_answer in src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *Context) completeTraceAnswer(root *ModelNode) (result ast.ASTRef, resultErr error) {
	constraints, err := c.unifiedTraceConstraints(root, true)
	if err != nil {
		return result, fmt.Errorf("pdr: unify complete proof trace: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, closeASTRefs(c.manager, constraints))
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	handles := make([]ast.ASTHandle, len(constraints))
	for index := range constraints {
		handles[index], err = c.manager.RefGet(constraints[index])
		if err != nil {
			return result, err
		}
	}
	formula, err := c.manager.MkAndArray(handles)
	if err != nil {
		return result, err
	}
	return c.manager.NewRef(formula)
}

// Proof reconstructs the latest reachable Horn derivation as a hyper-
// resolution proof. The asserted premises are the selected source rules; the
// conclusion is the complete bank-unified trace formula. The caller releases
// the returned reference through ASTManager.DelRef.
func (c *Context) Proof(queryDeclaration ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if err := c.open(); err != nil {
		return result, err
	}
	if !c.generateProofTrace || c.lastResult != lbool.True || c.queryDecl != queryDeclaration || c.query == nil {
		return result, ast.ErrASTDomain
	}
	root, found, err := c.search.Root()
	if err != nil || !found {
		if err != nil {
			return result, err
		}
		return result, ast.ErrASTDomain
	}
	if err = updateTraceModels(root); err != nil {
		return result, err
	}
	proofsEnabled, err := c.manager.ProofsEnabled()
	if err != nil {
		return result, err
	}
	if !proofsEnabled {
		if err = c.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
			return result, err
		}
		defer func() { resultErr = errors.Join(resultErr, c.manager.ToggleProofMode(ast.ASTProofDisabled)) }()
	}
	return c.reducedProofTrace(root)
}
