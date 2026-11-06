package pdr

// Source: context::reset in src/muz/pdr/pdr_context.cpp:1467-1488.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Reset discards the installed program, frames and query result while keeping
// the context's manager, configuration and reusable projection services alive.
// A new program can subsequently be installed with Initialize.
func (c *Context) Reset() error {
	if err := c.open(); err != nil {
		return err
	}
	c.unknownResult = false
	if err := c.search.Reset(); err != nil {
		return err
	}
	if err := closeTransformers(c.order, c.transformers); err != nil {
		return err
	}
	if err := c.owner.contexts.resetSolvers(true); err != nil {
		return err
	}
	c.transformers = nil
	c.order = nil
	c.auxiliary = make(map[ast.ASTHandle]bool)
	c.query = nil
	c.queryDecl = ast.ASTHandle{}
	c.lastResult = lbool.Undef
	c.inductive = 0
	c.farkasActive = false
	c.proofActive = false
	c.initialized = false
	return nil
}
