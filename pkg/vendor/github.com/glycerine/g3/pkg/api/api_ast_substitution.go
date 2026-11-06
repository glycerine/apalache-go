package api

// Source: src/api/api_ast.cpp:768-817.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// Substitute simultaneously replaces expressions in term. Every source and
// destination pair must have the same sort. The returned reference is owning
// and must be released with Manager.DelRef.
func (c *SMTContext) Substitute(term ast.ASTHandle, from, to []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if len(from) != len(to) {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	if _, err = m.GetKind(term); err != nil {
		return ast.ASTRef{}, err
	}
	for i := range from {
		fromSort, sortErr := m.GetSort(from[i])
		if sortErr != nil {
			return ast.ASTRef{}, sortErr
		}
		toSort, sortErr := m.GetSort(to[i])
		if sortErr != nil {
			return ast.ASTRef{}, sortErr
		}
		if fromSort != toSort {
			return ast.ASTRef{}, ast.ErrASTDomain
		}
	}

	substitution, err := m.NewExprSubstitution()
	if err != nil {
		return ast.ASTRef{}, err
	}
	replacer, err := rewriter.NewDefaultExprReplacer(m)
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelExprSubstitution(&substitution))
	}
	cleanup := func() error {
		return errors.Join(replacer.Close(), m.DelExprSubstitution(&substitution))
	}
	for i := range from {
		if err = m.ExprSubstitutionInsert(substitution, from[i], to[i], ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return ast.ASTRef{}, errors.Join(err, cleanup())
		}
	}
	if err = replacer.SetSubstitution(&substitution); err != nil {
		return ast.ASTRef{}, errors.Join(err, cleanup())
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, cleanup())
	}
	if err = replacer.Rewrite(term, result); err != nil {
		resultErr = errors.Join(err, cleanup(), m.DelRef(&result))
		return ast.ASTRef{}, resultErr
	}
	if err = cleanup(); err != nil {
		resultErr = errors.Join(err, m.DelRef(&result))
		return ast.ASTRef{}, resultErr
	}
	return result, nil
}

// SubstituteVariables replaces de Bruijn variable zero with replacements[0],
// variable one with replacements[1], and so on, matching Z3_substitute_vars.
// The returned reference is owning and must be released with Manager.DelRef.
func (c *SMTContext) SubstituteVariables(term ast.ASTHandle, replacements []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if _, err = m.GetKind(term); err != nil {
		return ast.ASTRef{}, err
	}
	substitution, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		return ast.ASTRef{}, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, substitution.Close())
	}
	if err = substitution.Substitute(term, replacements, result); err != nil {
		resultErr = errors.Join(err, substitution.Close(), m.DelRef(&result))
		return ast.ASTRef{}, resultErr
	}
	if err = substitution.Close(); err != nil {
		resultErr = errors.Join(err, m.DelRef(&result))
		return ast.ASTRef{}, resultErr
	}
	return result, nil
}
