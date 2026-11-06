package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// BoolElimination owns the rewritten formula and its Boolean realizer.
type BoolElimination struct {
	self       *BoolElimination
	manager    *ast.ASTManager
	Formula    ast.ASTRef
	Definition ast.ASTRef
	closed     bool
}

func (e *BoolElimination) Close() error {
	if e == nil || e.self != e || e.manager == nil {
		return ast.ErrASTHandle
	}
	if e.closed {
		return nil
	}
	var definitionErr, formulaErr error
	if e.Definition != (ast.ASTRef{}) {
		definitionErr = e.manager.DelRef(&e.Definition)
	}
	if e.Formula != (ast.ASTRef{}) {
		formulaErr = e.manager.DelRef(&e.Formula)
	}
	err := errors.Join(definitionErr, formulaErr)
	if err == nil {
		e.closed = true
	}
	return err
}

func (p *BoolPlugin) elimination(formula, definition ast.ASTHandle) (result *BoolElimination, resultErr error) {
	e := &BoolElimination{manager: p.manager}
	e.self = e
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, e.Close())
			result = nil
		}
	}()
	var err error
	e.Formula, err = p.manager.NewRef(formula)
	if err != nil {
		return nil, err
	}
	e.Definition, err = p.manager.NewRef(definition)
	if err != nil {
		return nil, err
	}
	return e, nil
}
