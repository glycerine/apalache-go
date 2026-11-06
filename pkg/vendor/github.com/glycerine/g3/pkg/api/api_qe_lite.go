package api

// Source: src/api/api_qe.cpp Z3_qe_lite.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
)

// QELite eliminates constants from body and returns the retained variables.
// The returned reference owns the projected body.
func (c *SMTContext) QELite(variables []ast.ASTHandle, body ast.ASTHandle) (result ast.ASTRef, remaining []ast.ASTHandle, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	if body.IsNull() {
		return ast.ASTRef{}, nil, ast.ErrASTHandle
	}
	if err = apiQEApplications(manager, variables); err != nil {
		return ast.ASTRef{}, nil, err
	}
	owned, err := apiQEOwnRefs(manager, variables)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, apiQECloseRefs(manager, &owned)) }()
	bodyRef, err := manager.NewRef(body)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&bodyRef)) }()
	config, err := c.apiQEConfig()
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	eliminator, err := qe.NewLite(manager, nil, true, config)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, eliminator.Close()) }()
	if err = eliminator.EliminateConstants(&owned, bodyRef); err != nil {
		return ast.ASTRef{}, nil, err
	}
	projected, err := manager.RefGet(bodyRef)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	result, err = manager.NewRef(projected)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	remaining = make([]ast.ASTHandle, 0, len(owned))
	for _, ref := range owned {
		value, x := manager.RefGet(ref)
		if x != nil {
			return ast.ASTRef{}, nil, errors.Join(x, manager.DelRef(&result))
		}
		remaining = append(remaining, value)
	}
	return result, remaining, nil
}
