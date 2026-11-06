package arith

// Source ownership: src/tactic/arith/bv2real_rewriter.h:35,146-148,205-208.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func (u *BV2RealUtil) borrowRewriter() error {
	if err := u.open(); err != nil {
		return err
	}
	if u.borrowers == ^uint32(0) {
		return arena.ErrBounds
	}
	u.borrowers++
	return nil
}

func (u *BV2RealUtil) releaseRewriter() error {
	if err := u.owner(); err != nil {
		return err
	}
	if u.borrowers == 0 {
		return ast.ErrASTPending
	}
	u.borrowers--
	return nil
}
