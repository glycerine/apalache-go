package arith

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *ForwardTrigPurifyTactic) Apply(goal *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || out == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = out.Prepare(t.manager); err != nil {
		return err
	}
	inv, err := newForwardTrigInvocation(t, goal)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, inv.close()) }()
	if err = inv.findUnsafe(); err != nil {
		return err
	}
	cfg := &forwardTrigConfig{invocation: inv}
	var walk *rewriter.Traversal
	if inv.proofs {
		walk, err = rewriter.NewProofTraversal(t.manager, cfg)
	} else {
		walk, err = rewriter.NewTraversal(t.manager, cfg)
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, walk.Close()) }()
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for j := uint32(0); j < size; j++ {
		current, e := goal.Form(j)
		if e != nil {
			return e
		}
		next, e := t.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		pr, e := t.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			_ = t.manager.DelRef(&next)
			return e
		}
		if inv.proofs {
			e = walk.RewriteWithProof(current, next, pr)
		} else {
			e = walk.Rewrite(current, next)
		}
		if e == nil {
			rewritten, x := t.manager.RefGet(next)
			if x != nil {
				e = x
			} else {
				proof := ast.ASTHandle{}
				if inv.proofs {
					old, x := goal.Proof(j)
					if x != nil {
						e = x
					} else if rp, x := t.manager.RefGet(pr); x != nil {
						e = x
					} else {
						proof, e = t.manager.MkProofModusPonens(old, rp)
					}
				}
				if e == nil {
					d, x := goal.Dependency(j)
					if x != nil {
						e = x
					} else {
						e = goal.Update(j, rewritten, proof, d)
					}
				}
			}
		}
		e = errors.Join(e, t.manager.DelRef(&pr), t.manager.DelRef(&next))
		if e != nil {
			return e
		}
	}
	if err = inv.publishConstraints(); err != nil {
		return err
	}
	if err = inv.publishModel(out); err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return out.AddGoal(goal)
}
