package qe

// Source: src/qe/qe.cpp:2541-2544 direct blast_or context add_var.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// AddVariable appends a borrowed application exactly as the direct source
// context appends to its caller-owned app_ref_vector.
func (c *SolverContext) AddVariable(variable ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	kind, err := c.manager.GetKind(variable)
	if err != nil || kind != ast.ASTApp {
		return errors.Join(ast.ErrASTDomain, err)
	}
	c.variables = append(c.variables, variable)
	c.variableOwners = append(c.variableOwners, ast.ASTRef{})
	c.contains = append(c.contains, nil)
	return nil
}

// adoptVariable transfers an existing reference into the context's variable
// vector. Ownership transfers only after the borrowed insertion succeeds.
func (c *SolverContext) adoptVariable(variable ast.ASTHandle, owner *ast.ASTRef) error {
	if err := c.open(); err != nil {
		return err
	}
	if owner == nil || *owner == (ast.ASTRef{}) {
		return ast.ErrASTHandle
	}
	h, err := c.manager.RefGet(*owner)
	if err != nil {
		return err
	}
	if h != variable {
		return ast.ErrASTDomain
	}
	if err = c.AddVariable(variable); err != nil {
		return err
	}
	c.variableOwners[len(c.variableOwners)-1] = *owner
	*owner = ast.ASTRef{}
	return nil
}

// Variables returns the current source order as a detached borrowed slice.
func (c *SolverContext) Variables() ([]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), c.variables...), nil
}
