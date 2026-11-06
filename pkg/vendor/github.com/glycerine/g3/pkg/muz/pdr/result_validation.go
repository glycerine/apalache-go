package pdr

// Source: context::validate_proof and context::validate_search in
// src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var errPDRResultValidation = errors.New("muz/pdr: result validation failed")

// validateSATFormula uses a fresh source-style SMT kernel. Validation must not
// borrow a PDR context whose assertion stack contains frames or reachability
// assumptions from the completed search.
func (c *Context) validateSATFormula(formula ast.ASTHandle) (result error) {
	if c == nil || c.owner == nil || c.owner.contexts == nil {
		return ErrContextOwner
	}
	contexts := c.owner.contexts
	if err := contexts.open(); err != nil {
		return err
	}
	attempt := newPDRValidationAttempt(c.manager)
	defer func() { result = finishPDRValidation(attempt, result) }()
	guard, err := c.manager.NewRef(formula)
	if err != nil {
		return err
	}
	attempt.references = append(attempt.references, guard)
	solver, err := smtcore.NewSMTSolver(c.manager, contexts.params, "", contexts.config)
	if err != nil {
		return err
	}
	attempt.solvers = append(attempt.solvers, solver)
	if err = solver.Assert(formula); err != nil {
		return err
	}
	status, err := solver.CheckSat(nil)
	if err != nil {
		return err
	}
	if status != lbool.True {
		return fmt.Errorf("%w: trace check returned %s", errPDRResultValidation, status)
	}
	return nil
}

func (c *Context) validateProofResult(queryDeclaration ast.ASTHandle) (result error) {
	attempt := newPDRValidationAttempt(c.manager)
	defer func() { result = finishPDRValidation(attempt, result) }()
	proof, err := c.Proof(queryDeclaration)
	if err != nil {
		return fmt.Errorf("pdr: construct validation proof: %w", err)
	}
	attempt.references = append(attempt.references, proof)
	proofHandle, err := c.manager.RefGet(proof)
	if err != nil {
		return err
	}
	return c.validateProofHandle(proofHandle)
}

func (c *Context) validateProofHandle(proofHandle ast.ASTHandle) (result error) {
	attempt := newPDRValidationAttempt(c.manager)
	defer func() { result = finishPDRValidation(attempt, result) }()
	checker, err := proofs.NewChecker(c.manager)
	if err != nil {
		return err
	}
	valid, sideConditions, err := checker.Check(proofHandle)
	attempt.sideConditions = sideConditions
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("%w: proof checker rejected reachable trace", errPDRResultValidation)
	}
	for index, condition := range sideConditions {
		// Source expr_ref tmp owns the negation before kernel construction.
		// Reserve its owner first so either allocation failure leaves the
		// temporary attached to the existing validation cleanup attempt.
		guard, x := c.manager.NewRef(ast.ASTHandle{})
		if x != nil {
			return x
		}
		attempt.references = append(attempt.references, guard)
		negated, x := c.manager.MkNot(condition)
		if !negated.IsNull() {
			x = errors.Join(x, c.manager.SetRef(guard, negated))
		}
		if x != nil {
			return x
		}
		contexts := c.owner.contexts
		solver, x := smtcore.NewSMTSolver(c.manager, contexts.params, "", contexts.config)
		if x != nil {
			return x
		}
		attempt.solvers = append(attempt.solvers, solver)
		if x = solver.Assert(negated); x == nil {
			var status lbool.LBool
			status, x = solver.CheckSat(nil)
			if x == nil && status != lbool.False {
				x = fmt.Errorf("%w: proof side condition %d check returned %s", errPDRResultValidation, index, status)
			}
		}
		if x != nil {
			return x
		}
	}
	return nil
}

// validateSATResult reconstructs the complete selected rule trace even when
// the configured answer is abbreviated. If proof traces are requested, it
// also checks the proof DAG and independently discharges every side condition.
func (c *Context) validateSATResult(queryDeclaration ast.ASTHandle) (result error) {
	if err := c.open(); err != nil {
		return err
	}
	if c.lastResult != lbool.True || c.query == nil || c.queryDecl != queryDeclaration || c.search == nil {
		return ast.ErrASTDomain
	}
	attempt := newPDRValidationAttempt(c.manager)
	defer func() { result = finishPDRValidation(attempt, result) }()
	root, found, err := c.search.Root()
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: reachable search has no selected root", errPDRResultValidation)
	}
	if c.generateProofTrace {
		if err = c.validateProofResult(queryDeclaration); err != nil {
			return fmt.Errorf("pdr: validate reachable proof: %w", err)
		}
	}
	trace, err := c.completeTraceAnswer(root)
	if err != nil {
		return fmt.Errorf("pdr: reconstruct validation trace: %w", err)
	}
	attempt.references = append(attempt.references, trace)
	traceHandle, err := c.manager.RefGet(trace)
	if err != nil {
		return err
	}
	if err = c.validateSATFormula(traceHandle); err != nil {
		return fmt.Errorf("pdr: validate reachable trace: %w", err)
	}
	return nil
}
