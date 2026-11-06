package arith

// Source ownership: src/tactic/arith/bv2int_rewriter.h:31,54,105-118.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func (r *BV2IntContext) borrowRewriter() error {
	if err := r.open(); err != nil {
		return err
	}
	if r.borrowers == ^uint32(0) {
		return arena.ErrBounds
	}
	r.borrowers++
	return nil
}

func (r *BV2IntContext) releaseRewriter() error {
	if err := r.owner(); err != nil {
		return err
	}
	if r.borrowers == 0 {
		return ast.ErrASTPending
	}
	r.borrowers--
	return nil
}
