package cmd_context

// Source: src/cmd_context/cmd_context.cpp result ownership/get_consequences and
// src/cmd_context/basic_cmds.cpp get-proof/get-unsat-core availability.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// Assertions returns the borrowed formulas in command-session order.
func (c *Context) Assertions() ([]ast.ASTHandle, error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(c.assertions))
	for i := range c.assertions {
		result[i] = c.assertions[i].expression
	}
	return result, nil
}

func (c *Context) ScopeLevel() (uint32, error) {
	if err := c.valid(); err != nil {
		return 0, err
	}
	return uint32(len(c.scopes)), nil
}

// Statistics returns an owning Go copy of the current solver statistics.
func (c *Context) Statistics() (statistics.Statistics, error) {
	if err := c.valid(); err != nil {
		return statistics.Statistics{}, err
	}
	if _, err := c.Manager(); err != nil {
		return statistics.Statistics{}, err
	}
	var result statistics.Statistics
	if err := c.checkSatResult().CollectStatistics(&result); err != nil {
		return statistics.Statistics{}, err
	}
	return result, nil
}

// Consequences replaces the prior command result and delegates to the
// installed solver's concrete consequence engine.
func (c *Context) Consequences(assumptions, variables []ast.ASTHandle) (status lbool.LBool, consequences []ast.ASTHandle, resultErr error) {
	if err := c.valid(); err != nil {
		return lbool.Undef, nil, err
	}
	if _, err := c.Manager(); err != nil {
		return lbool.Undef, nil, err
	}
	c.checked = false
	if err := c.clearCheckSatResult(); err != nil {
		return lbool.Undef, nil, err
	}
	status, consequences, resultErr = c.solver.GetConsequences(assumptions, variables)
	if resultErr != nil {
		return status, consequences, resultErr
	}
	c.status, c.checked = status, true
	return status, consequences, nil
}

// ImpliedEqualities partitions terms by equalities implied by the current
// assertions. The query invalidates access to the prior check result.
func (c *Context) ImpliedEqualities(terms []ast.ASTHandle) (lbool.LBool, []uint32, error) {
	if err := c.valid(); err != nil {
		return lbool.Undef, nil, err
	}
	if _, err := c.Manager(); err != nil {
		return lbool.Undef, nil, err
	}
	c.checked = false
	return smtcore.ImpliedEqualities(c.solver, terms)
}

// UnsatCore returns a borrowed copy of the most recent unsatisfiable result's
// tracked labels. The handles remain owned by the context result.
func (c *Context) UnsatCore() ([]ast.ASTHandle, error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	state, err := c.parameters.State()
	if err != nil {
		return nil, err
	}
	if !c.checked || c.status != lbool.False || !state.UnsatCore {
		return nil, fmt.Errorf("unsat core is not available")
	}
	return c.checkSatResult().UnsatCore()
}

// Proof returns the borrowed proof from the most recent unsatisfiable result.
// It remains owned by the context result and is invalidated by the next
// mutating solver operation.
func (c *Context) Proof() (ast.ASTHandle, error) {
	if err := c.valid(); err != nil {
		return ast.ASTHandle{}, err
	}
	state, err := c.parameters.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !state.Proof || !c.checked || c.status != lbool.False {
		return ast.ASTHandle{}, fmt.Errorf("proof is not available")
	}
	proof, err := c.checkSatResult().Proof()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if proof.IsNull() {
		return ast.ASTHandle{}, fmt.Errorf("proof is not available")
	}
	return proof, nil
}
