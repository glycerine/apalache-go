package bv

// Source: src/tactic/bv/bv_bound_chk_tactic.cpp bv_bound_chk_rewriter_cfg.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type bvBoundCheckConfiguration struct {
	rewriter.DefaultConfiguration
	owner               *BVBoundCheckTactic
	boolean             *rewriter.BoolRewriter
	pending             []*rewriter.RewriterParams
	limit               uint32
	maxSteps, maxMemory uint64
}

func (c *bvBoundCheckConfiguration) update(p *uparams.ParamsRef) (err error) {
	policy, err := rewriter.NewRewriterParams(c.owner.context, c.owner.globals, p)
	if err != nil {
		return err
	}
	defer func() {
		if e := policy.Close(); e != nil {
			c.pending = append(c.pending, policy)
			err = errors.Join(err, e)
		}
	}()
	c.limit, err = policy.BVIneqConsistencyTestMax()
	if err != nil {
		return err
	}
	n, err := policy.MaxMemory()
	if err != nil {
		return err
	}
	c.maxMemory = uint64(n) // Source bytes, without MB conversion.
	n, err = policy.MaxSteps()
	if err == nil {
		c.maxSteps = uint64(n)
	}
	return err
}
func (c *bvBoundCheckConfiguration) close() error {
	for len(c.pending) > 0 {
		i := len(c.pending) - 1
		if err := c.pending[i].Close(); err != nil {
			return err
		}
		c.pending = c.pending[:i]
	}
	if c.boolean != nil {
		if err := c.boolean.Close(); err != nil {
			return err
		}
		c.boolean = nil
	}
	return nil
}
func (c *bvBoundCheckConfiguration) RewritePatterns() bool                 { return false }
func (c *bvBoundCheckConfiguration) FlatAssoc(ast.ASTHandle) (bool, error) { return true, nil }
func (c *bvBoundCheckConfiguration) MaxStepsExceeded(n uint32) (bool, error) {
	if err := c.owner.services.Cooperation.Cooperate("bv-bound-chk"); err != nil {
		return false, err
	}
	if uint64(n) > c.maxSteps {
		return true, nil
	}
	allocated, err := c.owner.services.Memory.AllocationSize()
	if err != nil {
		return false, err
	}
	if allocated > c.maxMemory {
		return false, tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
	}
	return false, nil
}
func (c *bvBoundCheckConfiguration) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (status rewriter.RewriteStatus, err error) {
	m := c.owner.manager
	if err = m.ResetRef(proof); err != nil {
		return rewriter.RewriteFailed, err
	}
	family, err := m.GetFamilyID(d)
	if err != nil || family != m.BasicFamilyID() {
		return rewriter.RewriteFailed, err
	}
	bounds, err := rewriter.NewBVBounds(m)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	defer func() { err = errors.Join(err, bounds.Close()) }()
	status, err = bounds.Rewrite(c.limit, d, args, out)
	if err != nil || status == rewriter.RewriteFailed {
		return status, err
	}
	h, err := m.RefGet(out)
	if err != nil {
		return status, err
	}
	if m.IsFalse(h) || m.IsTrue(h) {
		c.owner.stats.Unsats++
	} else {
		singletons, err := bounds.Singletons()
		if err != nil {
			return status, err
		}
		if len(singletons) > 0 {
			c.owner.stats.Singletons++
		} else {
			kind, err := m.GetKind(h)
			if err != nil {
				return status, err
			}
			if kind == ast.ASTApp {
				n, err := m.GetNumArgs(h)
				if err != nil {
					return status, err
				}
				if n < args.Count() {
					c.owner.stats.Reduces++
				}
			}
		}
	}
	return status, c.owner.emitStep(d, args, h)
}
