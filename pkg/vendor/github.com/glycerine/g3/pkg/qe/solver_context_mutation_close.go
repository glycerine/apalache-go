package qe

// Source: src/qe/qe.cpp blast_or context reset and expr_ref destruction.

import "github.com/glycerine/g3/pkg/ast"

func (c *SolverContext) closeMutationState() error {
	for c.containsClosing < uint32(len(c.contains)) {
		contains := c.contains[c.containsClosing]
		if contains != nil {
			if err := contains.Close(); err != nil {
				return err
			}
			c.contains[c.containsClosing] = nil
		}
		c.containsClosing++
	}
	c.contains = nil
	if c.currentFormula != (ast.ASTRef{}) {
		if err := c.manager.DelRef(&c.currentFormula); err != nil {
			return err
		}
	}
	for len(c.variableOwners) > 0 {
		i := len(c.variableOwners) - 1
		if c.variableOwners[i] != (ast.ASTRef{}) {
			if err := c.manager.DelRef(&c.variableOwners[i]); err != nil {
				return err
			}
		}
		c.variableOwners = c.variableOwners[:i]
	}
	return nil
}
