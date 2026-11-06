package pdr

// Source: context::solve_impl, check_reachability, expand_node,
// expand_state, propagate, close_node, and the child-obligation portion of
// create_children in src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (c *Context) checkpoint() error {
	canceled, err := c.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return z3exception.NewDefaultException(c.manager.ResourceLimit().CancelMessage())
	}
	return nil
}

func (c *Context) closeNode(node *ModelNode) error {
	if err := node.SetClosed(); err != nil {
		return err
	}
	for parent := node.parent; parent != nil && parent.IsOneClosed(); parent = parent.parent {
		if err := parent.SetClosed(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Context) expandNode(node *ModelNode) (resultStatus lbool.LBool, resultErr error) {
	if node == nil || !node.IsOpen() {
		return lbool.Undef, ast.ErrASTDomain
	}
	if node.level < c.expanded {
		c.expanded = node.level
	}
	cached, err := node.transformer.IsCachedReachable(node.state)
	if err != nil {
		return lbool.Undef, err
	}
	if cached {
		return lbool.True, c.closeNode(node)
	}
	check, err := node.transformer.CheckReachable(node.level, node.state, true, true)
	if err != nil {
		return lbool.Undef, fmt.Errorf("muz/pdr: reachability check: %w", err)
	}
	if check.Status == lbool.True {
		if check.Model == nil {
			return lbool.Undef, ast.ErrASTDomain
		}
		if err = node.SetModel(check.Model); err != nil {
			return lbool.Undef, errors.Join(err, check.Model.DecRef())
		}
		if err = check.Model.DecRef(); err != nil {
			return lbool.Undef, err
		}
		if node.level == 0 {
			return lbool.True, c.closeNode(node)
		}
		if err = c.createChildrenProjected(node); err != nil {
			return lbool.Undef, fmt.Errorf("muz/pdr: projected child construction: %w", err)
		}
		return lbool.True, nil
	}
	if check.Status == lbool.False {
		cores, x := c.generalizeBlockingCores(node, check.Core, check.AssumesLevel)
		if x != nil {
			return lbool.Undef, fmt.Errorf("muz/pdr: blocking core generalization: %w", x)
		}
		defer func() { resultErr = errors.Join(resultErr, closeGeneralizedCores(c.manager, cores)) }()
		foundInvariant := false
		for _, core := range cores {
			lemmaRef, makeErr := c.owner.NotAndRef(core.literals)
			if makeErr != nil {
				return lbool.Undef, fmt.Errorf("muz/pdr: core negation: %w", makeErr)
			}
			lemma, makeErr := c.manager.RefGet(lemmaRef)
			if makeErr != nil {
				return lbool.Undef, errors.Join(makeErr, c.owner.releaseNotAndRef(&lemmaRef))
			}
			level := InfinityLevel
			if core.usesLevel {
				level = node.level
			} else {
				foundInvariant = true
			}
			_, makeErr = node.transformer.AddProperty(lemma, level)
			makeErr = errors.Join(makeErr, c.owner.releaseNotAndRef(&lemmaRef))
			if makeErr != nil {
				return lbool.Undef, fmt.Errorf("muz/pdr: blocking property: %w", makeErr)
			}
		}
		if node.level != 0 {
			if x = c.assertInvariants(node.level-1, 2098, "n.level() == 0 || check_invariant(n.level()-1)"); x != nil {
				return lbool.Undef, x
			}
		}
		if x = c.search.BacktrackLevel(!foundInvariant && c.flexibleTrace, node); x != nil {
			return lbool.Undef, x
		}
		return lbool.False, nil
	}
	return lbool.Undef, nil
}

func (c *Context) checkReachability(level uint32) (lbool.LBool, error) {
	truth, err := c.manager.MkTrue()
	if err != nil {
		return lbool.Undef, err
	}
	root, err := NewModelNode(nil, truth, c.query, level)
	if err != nil {
		return lbool.Undef, err
	}
	if err = c.search.SetRoot(root); err != nil {
		// Source set_root resets the prior search before adopting this node.
		// A refused reset leaves the newly constructed root owned here.
		if c.search.root != root {
			if cleanup := root.release(); cleanup != nil {
				// Keep its transformer alive through the search's existing
				// cleanup frontier if even the local destructor refuses.
				c.search.pendingRelease = append(c.search.pendingRelease, root)
				err = errors.Join(err, cleanup)
			}
		}
		return lbool.Undef, err
	}
	for {
		if x := c.checkpoint(); x != nil {
			return lbool.Undef, x
		}
		node, found, x := c.search.Next()
		if x != nil {
			return lbool.Undef, x
		}
		if !found {
			break
		}
		status, x := c.expandNode(node)
		if x != nil {
			return lbool.Undef, x
		}
		if status == lbool.Undef {
			return lbool.Undef, nil
		}
	}
	if root.IsClosed() {
		return lbool.True, nil
	}
	return lbool.False, nil
}

func (c *Context) propagate(maximum uint32) (bool, error) {
	if c.simplifyPre {
		if err := c.simplifyFormulas(); err != nil {
			return false, err
		}
	}
	for level := c.expanded; level <= maximum; level++ {
		if err := c.checkpoint(); err != nil {
			return false, err
		}
		all := true
		for _, declaration := range c.order {
			if err := c.checkpoint(); err != nil {
				return false, err
			}
			empty, err := c.transformers[declaration].PropagateToNextLevel(level)
			if err != nil {
				return false, err
			}
			all = empty && all
		}
		if err := c.assertInvariants(level, 2139, "check_invariant(lvl)"); err != nil {
			return false, err
		}
		if all && level == maximum {
			c.inductive = level
			return true, nil
		}
		if level == InfinityLevel {
			break
		}
	}
	if c.simplifyPost {
		if err := c.simplifyFormulas(); err != nil {
			return false, err
		}
	}
	return false, nil
}

// SetEmptyProgramUnsat records the source interface's trivial empty-program
// result without entering context::solve. The initialized program must be
// empty; in particular, this path emits no solve diagnostics and does not run
// post-solve validation.
func (c *Context) SetEmptyProgramUnsat(queryDeclaration ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if !c.initialized || queryDeclaration.IsNull() || len(c.order) != 0 || len(c.transformers) != 0 {
		return ast.ErrASTDomain
	}
	c.query = nil
	c.queryDecl = queryDeclaration
	c.lastResult = lbool.False
	c.unknownResult = false
	c.expanded, c.inductive = 0, 0
	return nil
}

// Solve runs the retained frame-expansion and propagation loop for the query
// predicate. Child projection preserves transition literals between predecessor
// states; native implicant minimization and QE generalization refine that cube.
func (c *Context) Solve(queryDeclaration ast.ASTHandle) (result lbool.LBool, resultErr error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	c.unknownResult = false
	// Register this before cleanup: a refused proof-mode restoration must
	// not publish an unknown answer for an unsuccessful solve.
	defer func() { c.unknownResult = result == lbool.Undef && resultErr == nil }()
	restoreProofMode, err := c.enterPDRProofMode()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		if restoreErr := restoreProofMode(); restoreErr != nil {
			c.lastResult = lbool.Undef
			result = lbool.Undef
			resultErr = errors.Join(resultErr, restoreErr)
		}
	}()
	if !c.initialized || c.search == nil {
		return lbool.Undef, ast.ErrASTDomain
	}
	if err = c.traceSolveStart(); err != nil {
		return lbool.Undef, err
	}
	c.query = nil
	c.queryDecl = queryDeclaration
	c.lastResult = lbool.Undef
	c.expanded, c.inductive = 0, 0
	query := c.transformers[queryDeclaration]
	if query == nil {
		if err = c.simplifyFormulas(); err != nil {
			return lbool.Undef, err
		}
		c.lastResult = lbool.False
		if err = c.traceUnreachableCertificate(); err == nil {
			err = c.verboseUnreachableCertificate()
		}
		if err != nil {
			c.lastResult = lbool.Undef
			return lbool.Undef, err
		}
		if err = c.finishValidatedResult(lbool.False); err != nil {
			return lbool.Undef, err
		}
		return c.lastResult, nil
	}
	c.query = query
	for level := uint32(0); ; level++ {
		if err = c.checkpoint(); err != nil {
			return lbool.Undef, err
		}
		c.expanded = level
		reachability, err := c.checkReachability(level)
		if err != nil {
			return lbool.Undef, err
		}
		if reachability == lbool.Undef {
			return lbool.Undef, nil
		}
		if reachability == lbool.True {
			if err = c.verboseReachableSearch(); err != nil {
				return lbool.Undef, err
			}
			if err = c.finishValidatedResult(lbool.True); err != nil {
				return lbool.Undef, err
			}
			if err = c.verboseReachableCertificate(); err != nil {
				c.lastResult = lbool.Undef
				return lbool.Undef, err
			}
			return c.lastResult, nil
		}
		if level != 0 {
			fixed, x := c.propagate(level)
			if x != nil {
				return lbool.Undef, x
			}
			if fixed {
				if x = c.simplifyFormulas(); x != nil {
					return lbool.Undef, x
				}
				c.lastResult = lbool.False
				if x = c.traceUnreachableCertificate(); x == nil {
					x = c.verboseUnreachableCertificate()
				}
				if x != nil {
					c.lastResult = lbool.Undef
					return lbool.Undef, x
				}
				for _, declaration := range c.order {
					if c.transformers[declaration] != c.query && c.inductive > 0 {
						if x = c.transformers[declaration].PropagateToInfinity(c.inductive); x != nil {
							return lbool.Undef, x
						}
					}
				}
				if x = c.finishValidatedResult(lbool.False); x != nil {
					return lbool.Undef, x
				}
				return c.lastResult, nil
			}
		}
		if level == InfinityLevel-1 {
			return lbool.Undef, nil
		}
	}
}

func (c *Context) simplifyFormulas() error {
	for _, declaration := range c.order {
		if err := c.transformers[declaration].SimplifyFormulas(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Context) LastResult() (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	return c.lastResult, nil
}
