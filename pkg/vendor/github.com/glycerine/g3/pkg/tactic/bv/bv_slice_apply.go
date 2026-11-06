package bv

// Source: src/ast/simplifiers/bv_slice.cpp reduce/process_eqs/apply_subst.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *BVSliceTactic) Apply(g *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || out == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	t.running = true
	defer func() { t.running = false }()
	if err = out.Prepare(t.manager); err != nil {
		return err
	}
	proofs, e := g.ProofsEnabled()
	if e != nil {
		return e
	}
	if proofs {
		return tactic.NewTacticException("bv-slice does not support proof generation")
	}
	frame := newBVSliceFrame(t)
	defer func() { err = errors.Join(err, frame.Close()) }()
	n, e := g.Size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		h, x := g.Form(i)
		if x != nil {
			return x
		}
		if x = frame.processEquality(h); x != nil {
			return x
		}
	}
	if len(frame.boundaries) > 0 {
		for i := uint32(0); i < n; i++ {
			h, x := g.Form(i)
			if x != nil {
				return x
			}
			value, x := frame.rewrite(h)
			if x != nil {
				return x
			}
			dep, x := g.Dependency(i)
			if x != nil {
				return x
			}
			if x = g.Update(i, value, ast.ASTHandle{}, dep); x != nil {
				return x
			}
		}
	}
	if e = g.IncDepth(); e != nil {
		return e
	}
	return out.AddGoal(g)
}
