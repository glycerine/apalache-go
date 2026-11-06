package qe

// Source: src/qe/qe.cpp:2529-2537 direct blast_or context m_fml update.

import "github.com/glycerine/g3/pkg/ast"

// CurrentFormula returns the latest formula installed by EliminateVariable.
// The returned handle is borrowed from the context-owned reference.
func (c *SolverContext) CurrentFormula() (ast.ASTHandle, bool, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if c.currentFormula == (ast.ASTRef{}) {
		return ast.ASTHandle{}, false, nil
	}
	handle, err := c.manager.RefGet(c.currentFormula)
	return handle, err == nil, err
}
