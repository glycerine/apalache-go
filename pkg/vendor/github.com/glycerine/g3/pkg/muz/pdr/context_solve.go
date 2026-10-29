package pdr

// Source: context::solve_impl, check_reachability, expand_node,
// expand_state, propagate, close_node, and the child-obligation portion of
// create_children in src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

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
			lemma, makeErr := c.owner.NotAnd(core.literals)
			if makeErr != nil {
				return lbool.Undef, fmt.Errorf("muz/pdr: core negation: %w", makeErr)
			}
			level := InfinityLevel
			if core.usesLevel {
				level = node.level
			} else {
				foundInvariant = true
			}
			if _, makeErr = node.transformer.AddProperty(lemma, level); makeErr != nil {
				return lbool.Undef, fmt.Errorf("muz/pdr: blocking property: %w", makeErr)
			}
		}
		if x = c.search.BacktrackLevel(!foundInvariant && c.flexibleTrace, node); x != nil {
			return lbool.Undef, x
		}
		return lbool.False, nil
	}
	return lbool.Undef, nil
}

func (c *Context) checkReachability(level uint32) (bool, error) {
	truth, err := c.manager.MkTrue()
	if err != nil {
		return false, err
	}
	root, err := NewModelNode(nil, truth, c.query, level)
	if err != nil {
		return false, err
	}
	if err = c.search.SetRoot(root); err != nil {
		return false, err
	}
	for {
		node, found, x := c.search.Next()
		if x != nil {
			return false, x
		}
		if !found {
			break
		}
		status, x := c.expandNode(node)
		if x != nil {
			return false, x
		}
		if status == lbool.Undef {
			return false, nil
		}
	}
	return root.IsClosed(), nil
}

func (c *Context) propagate(maximum uint32) (bool, error) {
	for level := c.expanded; level <= maximum; level++ {
		all := true
		for _, declaration := range c.order {
			empty, err := c.transformers[declaration].PropagateToNextLevel(level)
			if err != nil {
				return false, err
			}
			all = empty && all
		}
		if all && level == maximum {
			c.inductive = level
			return true, nil
		}
		if level == InfinityLevel {
			break
		}
	}
	return false, nil
}

// Solve runs the retained frame-expansion and propagation loop for the query
// predicate. Child projection preserves transition literals between predecessor
// states; native implicant minimization and QE generalization refine that cube.
func (c *Context) Solve(queryDeclaration ast.ASTHandle) (result lbool.LBool, resultErr error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	restoreProofMode, err := c.enterPDRProofMode()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, restoreProofMode()) }()
	if !c.initialized || c.search == nil {
		return lbool.Undef, ast.ErrASTDomain
	}
	c.query = nil
	c.queryDecl = queryDeclaration
	c.lastResult = lbool.Undef
	c.expanded, c.inductive = 0, 0
	query := c.transformers[queryDeclaration]
	if query == nil {
		c.lastResult = lbool.False
		return c.lastResult, nil
	}
	c.query = query
	for level := uint32(0); ; level++ {
		c.expanded = level
		reachable, err := c.checkReachability(level)
		if err != nil {
			return lbool.Undef, err
		}
		if reachable {
			c.lastResult = lbool.True
			return c.lastResult, nil
		}
		if level != 0 {
			fixed, x := c.propagate(level)
			if x != nil {
				return lbool.Undef, x
			}
			if fixed {
				c.lastResult = lbool.False
				for _, declaration := range c.order {
					if c.transformers[declaration] != c.query && c.inductive > 0 {
						if x = c.transformers[declaration].PropagateToInfinity(c.inductive); x != nil {
							return lbool.Undef, x
						}
					}
				}
				return c.lastResult, nil
			}
		}
		if level == InfinityLevel-1 {
			return lbool.Undef, nil
		}
	}
}

func (c *Context) LastResult() (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	return c.lastResult, nil
}
