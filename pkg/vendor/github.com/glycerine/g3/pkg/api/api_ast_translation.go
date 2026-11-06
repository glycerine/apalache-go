package api

// Source: src/api/api_ast.cpp:1230-1249.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// TranslateAST copies value into target. The returned reference is owned by
// target's manager and must be released through that manager's DelRef before
// target is closed.
func (c *SMTContext) TranslateAST(value ast.ASTHandle, target *SMTContext) (result ast.ASTRef, resultErr error) {
	if c == target || (c != nil && target != nil && c.smtContextState == target.smtContextState) {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	from, err := c.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	to, err := target.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if _, err = from.GetKind(value); err != nil {
		return ast.ASTRef{}, err
	}
	translator, err := ast.NewASTTranslation(from, to, true)
	if err != nil {
		return ast.ASTRef{}, err
	}
	translated, err := translator.Translate(value)
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, translator.Close())
	}
	result, err = to.NewRef(translated)
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, translator.Close())
	}
	if err = translator.Close(); err != nil {
		resultErr = errors.Join(err, to.DelRef(&result))
		return ast.ASTRef{}, resultErr
	}
	return result, nil
}
