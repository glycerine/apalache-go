package bv

// Source: src/tactic/bv/max_bv_sharing_tactic.cpp:29-271.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type sharingConfiguration struct {
	rewriter.DefaultConfiguration
	pass    *SharingGoalPass
	reducer *SharingReducer
}

func (c *sharingConfiguration) MaxStepsExceeded(n uint32) (bool, error) {
	p := c.pass
	if p.cooperation != nil {
		if err := p.cooperation.Cooperate("max bv sharing"); err != nil {
			return false, err
		}
	}
	used, err := p.memory.AllocationSize()
	if err != nil {
		return false, err
	}
	if used > p.maxMemory {
		return false, tactic.NewTacticException(ErrSharingMaxMemory.Error())
	}
	return n > p.maxSteps, nil
}

func (c *sharingConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	// The reducer records pairs even on the binary BR_FAILED branch.
	values := make([]ast.ASTHandle, args.Count())
	for i := range values {
		var err error
		values[i], err = args.At(uint32(i))
		if err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	h, changed, err := c.reducer.ReduceApp(decl, values)
	if err != nil || !changed {
		return rewriter.RewriteFailed, err
	}
	if err = c.pass.manager.ResetRef(proof); err == nil {
		err = c.pass.manager.SetRef(out, h)
	}
	return rewriter.RewriteDone, err
}

type sharingImplementation struct {
	configuration sharingConfiguration
	traversal     *rewriter.Traversal
	out, proof    ast.ASTRef
	steps         uint32
	proofGen      bool
}

func newSharingImplementation(p *SharingGoalPass) (*sharingImplementation, error) {
	if p == nil || p.manager == nil {
		return nil, ast.ErrASTHandle
	}
	r := &sharingImplementation{configuration: sharingConfiguration{pass: p}}
	var err error
	r.configuration.reducer, err = NewSharingReducer(p.manager, p.family, p.maxArgs)
	if err == nil {
		var proofs bool
		proofs, err = p.manager.ProofsEnabled()
		r.proofGen = proofs
		if err == nil && proofs {
			r.traversal, err = rewriter.NewProofTraversal(p.manager, &r.configuration)
		} else if err == nil {
			r.traversal, err = rewriter.NewTraversal(p.manager, &r.configuration)
		}
	}
	if err == nil {
		r.out, err = p.manager.NewRef(ast.ASTHandle{})
	}
	if err == nil {
		r.proof, err = p.manager.NewRef(ast.ASTHandle{})
	}
	if err != nil {
		return nil, errors.Join(err, r.Close())
	}
	return r, nil
}

func (r *sharingImplementation) Close() error {
	if r == nil {
		return nil
	}
	m := r.configuration.pass.manager
	for _, ref := range []*ast.ASTRef{&r.proof, &r.out} {
		if *ref != (ast.ASTRef{}) {
			if err := m.DelRef(ref); err != nil {
				return err
			}
		}
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
		r.traversal = nil
	}
	if r.configuration.reducer != nil {
		if err := r.configuration.reducer.Close(); err != nil {
			return err
		}
		r.configuration.reducer = nil
	}
	return nil
}

func (r *sharingImplementation) apply(goal *tactic.Goal) (err error) {
	m := r.configuration.pass.manager
	if goal == nil || goal.Manager() != m {
		return ast.ErrASTHandle
	}
	if ast.ASTSourceDebug {
		if ok, e := goal.IsWellSorted(); e != nil {
			return e
		} else if !ok {
			return ast.ErrASTDomain
		}
	}
	report, err := tactic.NewTacticReport("max-bv-sharing", goal)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		bad, e := goal.Inconsistent()
		if e != nil {
			return e
		}
		if bad {
			break
		}
		form, e := goal.Form(i)
		if e != nil {
			return e
		}
		if r.proofGen {
			e = r.traversal.RewriteWithProof(form, r.out, r.proof)
		} else {
			e = r.traversal.Rewrite(form, r.out)
		}
		if e != nil {
			return e
		}
		steps, e := r.traversal.NumSteps()
		if e != nil {
			return e
		}
		r.steps += steps
		if proofs {
			original, e := goal.Proof(i)
			if e != nil {
				return e
			}
			rewrite, e := m.RefGet(r.proof)
			if e != nil {
				return e
			}
			combined, e := m.MkProofModusPonens(original, rewrite)
			if e != nil {
				return e
			}
			if e = m.SetRef(r.proof, combined); e != nil {
				return e
			}
		}
		updated, e := m.RefGet(r.out)
		if e != nil {
			return e
		}
		proof, e := m.RefGet(r.proof)
		if e != nil {
			return e
		}
		dep, e := goal.Dependency(i)
		if e != nil {
			return e
		}
		if e = goal.Update(i, updated, proof, dep); e != nil {
			return e
		}
	}
	if err = r.configuration.reducer.clearPairs(); err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	tags := r.configuration.reducer.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err = tags.EmitConditional("max_bv_sharing", "operator()", "src/tactic/bv/max_bv_sharing_tactic.cpp", 268, func() bool { return true }, func(w io.Writer) error { return goal.Display(w) })
	if tags.IsStreamFailure(err) {
		err = nil
	}
	if err == nil && ast.ASTSourceDebug {
		if ok, e := goal.IsWellSorted(); e != nil {
			err = e
		} else if !ok {
			err = ast.ErrASTDomain
		}
	}
	return err
}
