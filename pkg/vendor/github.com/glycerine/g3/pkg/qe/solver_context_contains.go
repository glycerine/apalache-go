package qe

// Source: src/qe/qe.cpp:2524-2527 and blast_or init/reset contains ownership.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// Contains returns the cached contains_app for the current variable at index.
// The cache follows variables when EliminateVariable applies source swap-pop.
func (c *SolverContext) Contains(index uint32) (*ast.ContainsApp, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if index >= uint32(len(c.variables)) {
		return nil, arena.ErrBounds
	}
	for len(c.contains) < len(c.variables) {
		c.contains = append(c.contains, nil)
	}
	if c.contains[index] != nil {
		return c.contains[index], nil
	}
	contains, err := ast.NewContainsApp(c.manager, c.variables[index])
	if err != nil {
		return nil, err
	}
	c.contains[index] = contains
	return contains, nil
}
