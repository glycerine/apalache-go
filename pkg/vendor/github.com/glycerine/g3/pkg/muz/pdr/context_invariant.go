package pdr

// Source: pdr_context.cpp context::check_invariant, lines 2366-2390.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// CheckPredicateInvariant verifies that one predicate's next frame is implied
// by its source transition premises at level.
func (c *Context) CheckPredicateInvariant(level uint32, declaration ast.ASTHandle) (valid bool, resultErr error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if level == InfinityLevel {
		return false, ast.ErrASTDomain
	}
	transformer := c.transformers[declaration]
	if transformer == nil {
		return false, ast.ErrASTDomain
	}
	attempt := newPDRValidationAttempt(c.manager)
	defer func() { resultErr = finishPDRValidation(attempt, resultErr) }()
	invariantGuard, err := transformer.FormulasRef(level+1, false)
	if err != nil {
		return false, err
	}
	attempt.references = append(attempt.references, invariantGuard)
	invariant, err := c.manager.RefGet(invariantGuard)
	if err != nil {
		return false, err
	}
	if c.manager.IsTrue(invariant) {
		return true, nil
	}
	premises, err := transformer.Premises(level)
	if err != nil {
		return false, err
	}
	negated, err := c.manager.MkNot(invariant)
	if err != nil {
		return false, err
	}
	premises = append(premises, negated)
	formula, err := c.manager.MkAndArray(premises)
	if err != nil {
		return false, err
	}
	guard, err := c.manager.NewRef(formula)
	if err != nil {
		return false, err
	}
	attempt.references = append(attempt.references, guard)
	contexts := c.owner.contexts
	solver, err := smtcore.NewSMTSolver(c.manager, contexts.params, "", contexts.config)
	if err != nil {
		return false, err
	}
	attempt.solvers = append(attempt.solvers, solver)
	if err = solver.Assert(formula); err != nil {
		return false, err
	}
	status, err := solver.CheckSat(nil)
	return status == lbool.False, err
}

// CheckInvariants verifies every live predicate transformer at level.
func (c *Context) CheckInvariants(level uint32) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	for _, declaration := range c.order {
		if err := c.checkpoint(); err != nil {
			return false, err
		}
		valid, err := c.CheckPredicateInvariant(level, declaration)
		if err != nil || !valid {
			return valid, err
		}
	}
	return true, nil
}

func (c *Context) assertInvariants(level uint32, line int32, condition string) error {
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := debug.ConditionalAssertionEnabled("pdr")
	if err != nil || !enabled {
		return err
	}
	valid, err := c.CheckInvariants(level)
	if err != nil {
		return err
	}
	if valid {
		return nil
	}
	return diagnostic.DebugAssertionFailure(debug, "src/muz/pdr/pdr_context.cpp", line, condition)
}
