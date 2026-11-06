package qe

// Source: src/qe/qe.cpp:2529-2537 direct blast_or context elim_var.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// EliminateVariable installs formula and removes index using the source's
// last-element swap. Definition is intentionally ignored by this direct host,
// matching the blast_or implementation.
func (c *SolverContext) EliminateVariable(index uint32, formula, definition ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if index >= uint32(len(c.variables)) {
		return arena.ErrBounds
	}
	kind, err := c.manager.GetKind(formula)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp && kind != ast.ASTVar && kind != ast.ASTQuantifier {
		return ast.ErrASTDomain
	}
	next, err := c.manager.NewRef(formula)
	if err != nil {
		return err
	}
	if c.currentFormula != (ast.ASTRef{}) {
		if err = c.manager.DelRef(&c.currentFormula); err != nil {
			_ = c.manager.DelRef(&next)
			return err
		}
	}
	c.currentFormula = next
	last := len(c.variables) - 1
	if len(c.contains) < len(c.variables) {
		for len(c.contains) < len(c.variables) {
			c.contains = append(c.contains, nil)
		}
	}
	removed := c.contains[index]
	if removed != nil {
		if err = removed.Close(); err != nil {
			return err
		}
	}
	if c.variableOwners[index] != (ast.ASTRef{}) {
		if err = c.manager.DelRef(&c.variableOwners[index]); err != nil {
			return err
		}
	}
	c.variableOwners[index] = c.variableOwners[last]
	c.variableOwners[last] = ast.ASTRef{}
	c.variableOwners = c.variableOwners[:last]
	c.variables[index] = c.variables[last]
	c.variables = c.variables[:last]
	c.contains[index] = c.contains[last]
	c.contains[last] = nil
	c.contains = c.contains[:last]
	return nil
}
