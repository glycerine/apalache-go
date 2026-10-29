package spacer

// Source: pred_transformer::get_reachable in
// src/muz/spacer/spacer_context.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// Reachable returns the disjunction of accumulated reach facts with local
// state constants replaced by predicate-column variables.
func (p *PredicateTransformer) Reachable() (result ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return result, err
	}
	facts := make([]ast.ASTHandle, len(p.reachFacts))
	for index, fact := range p.reachFacts {
		auxiliaries, err := fact.Auxiliaries()
		if err != nil {
			return result, err
		}
		if len(auxiliaries) != 0 {
			return result, ast.ErrASTDomain
		}
		facts[index], err = fact.Fact()
		if err != nil {
			return result, err
		}
	}
	formula, err := p.manager.MkOrArray(facts)
	if err != nil {
		return result, err
	}
	substitution, err := p.manager.NewExprSubstitution()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelExprSubstitution(&substitution)) }()
	for index, signature := range p.signature {
		declaration, x := p.manager.RefGet(signature)
		if x != nil {
			return result, x
		}
		current, x := p.owner.OldToNew(declaration, 0)
		if x != nil {
			return result, x
		}
		constant, x := p.manager.MkApp(current, nil)
		if x != nil {
			return result, x
		}
		sort, x := p.manager.GetRange(declaration)
		if x != nil {
			return result, x
		}
		variable, x := p.manager.MkVar(uint32(index), sort)
		if x != nil {
			return result, x
		}
		if x = p.manager.ExprSubstitutionInsert(substitution, constant, variable, ast.ASTHandle{}, ast.ASTDependency{}); x != nil {
			return result, x
		}
	}
	replacer, err := rewriter.NewDefaultExprReplacer(p.manager)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, replacer.Close()) }()
	if err = replacer.SetSubstitution(&substitution); err != nil {
		return result, err
	}
	result, err = p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = replacer.Rewrite(formula, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, p.manager.DelRef(&result))
	}
	return result, nil
}
