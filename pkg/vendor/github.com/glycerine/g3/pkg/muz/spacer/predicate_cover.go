package spacer

// Source: pred_transformer::add_cover and pred_transformer::get_cover_delta
// in src/muz/spacer/spacer_context.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// AddCover installs a property expressed over the predicate's bound
// variables. Level -1 denotes the invariant frame.
func (p *PredicateTransformer) AddCover(level int, property ast.ASTHandle) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if level < -1 {
		return ast.ErrASTDomain
	}
	arguments := make([]ast.ASTHandle, len(p.signature))
	for index, signature := range p.signature {
		declaration, err := p.manager.RefGet(signature)
		if err != nil {
			return err
		}
		current, err := p.owner.OldToNew(declaration, 0)
		if err != nil {
			return err
		}
		arguments[index], err = p.manager.MkApp(current, nil)
		if err != nil {
			return err
		}
	}
	substitution, err := rewriter.NewVarSubstitution(p.manager, false)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	converted, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&converted)) }()
	if err = substitution.Substitute(property, arguments, converted); err != nil {
		return err
	}
	handle, err := p.manager.RefGet(converted)
	if err != nil {
		return err
	}
	conjuncts, err := p.manager.FlattenAnd([]ast.ASTHandle{handle})
	if err != nil {
		return err
	}
	target := InfinityLevel
	if level >= 0 {
		target = uint32(level)
	}
	for _, conjunct := range conjuncts {
		lemma, err := NewLemma(p.owner, conjunct, target)
		if err != nil {
			return err
		}
		_, err = p.frames.AddLemma(lemma)
		if err != nil {
			return errors.Join(err, lemma.Close())
		}
	}
	return nil
}

// CoverDelta returns one exact frame, translated from local state constants
// back to bound variables. Level -1 denotes the invariant frame.
func (p *PredicateTransformer) CoverDelta(level int) (result ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return result, err
	}
	if level < -1 {
		return result, ast.ErrASTDomain
	}
	target := InfinityLevel
	if level >= 0 {
		target = uint32(level)
	}
	lemmas, err := p.frames.FrameLemmas(target)
	if err != nil {
		return result, err
	}
	formula, err := p.manager.MkAndArray(lemmas)
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
