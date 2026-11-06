package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:716.

import "github.com/glycerine/g3/pkg/ast"

// demodulatorFunction validates the classifier's large-term invariant before
// the declaration lookup that follows the source assertion.
func (r *formulaProcessor) demodulatorFunction(large ast.ASTHandle) (ast.ASTHandle, error) {
	kind, e := r.manager.GetKind(large)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if kind != ast.ASTApp {
		if enabled, e := forwardAssertionEnabled(); e != nil {
			return ast.ASTHandle{}, e
		} else if enabled {
			if e := forwardAssertionFailure(716, "is_app(large)"); e != nil {
				return ast.ASTHandle{}, e
			}
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return r.manager.GetDecl(large)
}
