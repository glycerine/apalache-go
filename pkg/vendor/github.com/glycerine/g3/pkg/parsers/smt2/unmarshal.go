package smt2

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
)

// Unmarshal parses an SMT-LIB benchmark and returns an owning reference to the
// conjunction of its final assertions in manager. The caller must release the
// result through manager.DelRef.
func Unmarshal(input io.Reader, manager *ast.ASTManager) (result ast.ASTRef, resultErr error) {
	if input == nil || manager == nil {
		return ast.ASTRef{}, ErrMarshalInput
	}
	parsed, err := cmd_context.NewAuxiliaryContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, parsed.Close()) }()
	if err = ParseAssertions(parsed, input); err != nil {
		return ast.ASTRef{}, err
	}
	source, err := parsed.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	assertions, err := parsed.Assertions()
	if err != nil {
		return ast.ASTRef{}, err
	}
	conjunction, err := source.MkAndArray(assertions)
	if err != nil {
		return ast.ASTRef{}, err
	}
	translation, err := ast.NewASTTranslation(source, manager, true)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, translation.Close()) }()
	translated, err := translation.Translate(conjunction)
	if err != nil {
		return ast.ASTRef{}, err
	}
	result, err = manager.NewRef(translated)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return result, nil
}
