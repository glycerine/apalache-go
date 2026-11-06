package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp imp::operator().

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *BlastTermITETactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	t.running = true
	defer func() { t.running = false }()
	if sorted, e := goal.IsWellSorted(); e != nil {
		return e
	} else if !sorted {
		return ast.ErrASTDomain
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("blast-term-ite", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = t.traversal.Reset(); err != nil {
		return err
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	out, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&out)) }()
	proofOut, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&proofOut)) }()
	n, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		current, e := goal.Form(i)
		if e != nil {
			return e
		}
		if proofs {
			e = t.traversal.RewriteWithProof(current, out, proofOut)
		} else {
			e = t.traversal.Rewrite(current, out)
		}
		if e != nil {
			return e
		}
		rewritten, e := t.manager.RefGet(out)
		if e != nil {
			return e
		}
		var proof ast.ASTHandle
		if proofs {
			rewriteProof, e := t.manager.RefGet(proofOut)
			if e != nil {
				return e
			}
			oldProof, e := goal.Proof(i)
			if e != nil {
				return e
			}
			proof, e = t.manager.MkProofModusPonens(oldProof, rewriteProof)
			if e != nil {
				return e
			}
		}
		dependency, e := goal.Dependency(i)
		if e != nil {
			return e
		}
		if e = goal.Update(i, rewritten, proof, dependency); e != nil {
			return e
		}
	}
	if err = tactic.ReportTacticProgress(":blast-term-ite-consts", t.numFresh); err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}
