package fp

// Source: src/muz/base/dl_engine_base.h engine_base::query and
// src/muz/base/dl_rule.cpp rule_manager::mk_query.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// makeSpacerRelationQuery creates a local synthetic rule without changing the
// persistent program. Its predicate remains registered for model filtering.
func (e *PropositionalEngine) makeSpacerRelationQuery(declaration ast.ASTHandle, generateProof bool) (queryDeclaration ast.ASTHandle, result *base.Rule, resultErr error) {
	queryRef, err := e.makeRelationQueryExpression(declaration)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	attempt := &spacerRelationQueryAttempt{manager: e.manager, roots: []ast.ASTRef{queryRef}}
	defer func() { queryDeclaration, result, resultErr = attempt.finish(queryDeclaration, resultErr) }()
	query, err := e.manager.RefGet(queryRef)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	queryDeclaration, headRef, err := e.makeQueryHead(query)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	attempt.roots = append(attempt.roots, headRef)
	head, err := e.manager.RefGet(headRef)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	if err = e.rules.RegisterPredicate(queryDeclaration); err != nil {
		return ast.ASTHandle{}, nil, err
	}
	e.auxiliary[queryDeclaration] = true
	attempt.rule, err = e.rules.NewRule(head, []ast.ASTHandle{query}, nil, "")
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	if generateProof {
		if err = e.rules.MkRuleAssertedProof(attempt.rule); err != nil {
			return ast.ASTHandle{}, nil, err
		}
	}
	return queryDeclaration, attempt.rule, nil
}
