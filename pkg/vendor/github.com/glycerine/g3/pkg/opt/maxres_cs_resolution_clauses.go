package opt

// Source: src/opt/maxres.cpp cs_max_resolve dual clauses.

import "github.com/glycerine/g3/pkg/ast"

func maxResCorrectionClauses(owner *maxSMTASTOwner, correction []ast.ASTHandle) ([]ast.ASTHandle, error) {
	return maxResCorrectionClausesWithProxy(owner, nil, correction)
}

func maxResCorrectionClausesForEngine(e *maxResEngine, correction []ast.ASTHandle) ([]ast.ASTHandle, error) {
	return maxResCorrectionClausesWithProxy(&e.owner, e, correction)
}

func maxResCorrectionClausesWithProxy(owner *maxSMTASTOwner, engine *maxResEngine, correction []ast.ASTHandle) ([]ast.ASTHandle, error) {
	if len(correction) == 0 {
		return nil, errMaxResState
	}
	falsehood, err := owner.manager.MkFalse()
	if err != nil {
		return nil, err
	}
	disjunction := falsehood
	clauses := make([]ast.ASTHandle, 0, len(correction)-1)
	for i := 1; i < len(correction); i++ {
		clausePrefix, cause := owner.hold(owner.manager.MkOr(correction[i-1], disjunction))
		if cause != nil {
			return nil, cause
		}
		if i > 2 {
			if engine == nil {
				disjunction = clausePrefix
			} else {
				disjunction, err = engine.freshProxy(clausePrefix, false, "d")
			}
		} else {
			disjunction = clausePrefix
		}
		if err != nil {
			return nil, err
		}
		clause, err := owner.hold(owner.manager.MkAnd(correction[i], clausePrefix))
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	return clauses, nil
}
