package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp rw_cfg policy callbacks.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type blastTermITEConfiguration struct {
	rewriter.DefaultConfiguration
	owner      *BlastTermITETactic
	standalone bool
}

func (c *blastTermITEConfiguration) RewritePatterns() bool { return false }
func (c *blastTermITEConfiguration) FlatAssoc(ast.ASTHandle) (bool, error) {
	return false, nil
}
func (c *blastTermITEConfiguration) MaxStepsExceeded(uint32) (bool, error) {
	if c.owner == nil || c.standalone {
		return false, nil
	}
	if err := c.owner.services.Cooperation.Cooperate("blast term ite"); err != nil {
		return false, err
	}
	// The source retains max_memory but its allocation-size check is commented
	// out. Preserve that behavior instead of introducing a new refusal.
	return false, nil
}
