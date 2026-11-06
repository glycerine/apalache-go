package spacer

// Source: src/muz/spacer/spacer_context.h context::set_unsat and
// src/muz/spacer/spacer_dl_interface.cpp's empty transformed-program branch.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// SetEmptyProgramUnsat records the interface's trivial empty-program result
// without entering Context.Solve. Query and rule installation must already be
// complete so model and answer export keep their ordinary owners.
func (c *Context) SetEmptyProgramUnsat() error {
	if err := c.open(); err != nil {
		return err
	}
	if !c.initialized || c.queryDecl.IsNull() || c.query == nil || c.solved {
		return ast.ErrASTDomain
	}
	for _, transformer := range c.transformers {
		if transformer == nil || len(transformer.rules) != 0 || len(transformer.uses) != 0 {
			return ast.ErrASTDomain
		}
	}
	c.solved = true
	c.lastResult = lbool.False
	c.inductive = 0
	c.bounded = false
	return nil
}
