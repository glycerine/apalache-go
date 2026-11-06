package spacer

// Source: pred_transformer::get_reachable in
// src/muz/spacer/spacer_context.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// Reachable returns the disjunction of accumulated reach facts with local
// state constants replaced by predicate-column variables.
func (p *PredicateTransformer) Reachable() (result ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return result, err
	}
	attempt := &reachableExportAttempt{manager: p.manager, runtime: p.manager.NumeralService().RuntimeOwner()}
	defer func() { attempt.result = result; result, resultErr = attempt.finish(resultErr) }()
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
	if len(facts) == 0 {
		falsehood, err := p.manager.MkFalse()
		if err != nil {
			return result, err
		}
		return p.manager.NewRef(falsehood)
	}
	substitution, err := p.manager.NewExprSubstitution()
	if err != nil {
		return result, err
	}
	attempt.substitution = substitution
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
	config := p.owner.contexts[0].config
	replacer, err := rewriter.NewSimplifyingExprReplacer(p.manager, config.Params, config.Globals, nil)
	attempt.replacer = replacer
	if err != nil {
		return result, err
	}
	if err = replacer.SetSubstitution(&substitution); err != nil {
		return result, err
	}
	// Source rewrites each fact before mk_or. Rewriting the combined
	// disjunction would simplify across facts and change its output shape.
	for index, fact := range facts {
		ref, err := p.manager.NewRef(fact)
		if err != nil {
			return result, err
		}
		attempt.rewritten = append(attempt.rewritten, ref)
		if err = replacer.Rewrite(fact, ref); err != nil {
			return result, err
		}
		facts[index], err = p.manager.RefGet(ref)
		if err != nil {
			return result, err
		}
	}
	formula, err := p.manager.MkOrArray(facts)
	if err != nil {
		return result, err
	}
	return p.manager.NewRef(formula)
}
