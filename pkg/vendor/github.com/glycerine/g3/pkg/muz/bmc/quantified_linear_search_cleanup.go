package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp qlinear::check solver scope and
// winning model lifetime.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
)

type quantifiedLinearSearchAttempt struct {
	manager       *ast.ASTManager
	solver        *smtcore.SMTSolver
	target        uint32
	pushed        bool
	nested        memory.ConstructionFailure
	terms         *quantifiedLinear
	winning       *model.ModelCore
	modelConsumed bool
}

func (a *quantifiedLinearSearchAttempt) begin() error {
	var err error
	a.target, err = a.solver.ScopeLevel()
	if err != nil {
		return err
	}
	// The native solver reports whether a failed Push published its scope.
	// Cleanup reconciles that observed depth with the saved caller depth.
	a.pushed = true
	return a.solver.Push()
}

func (a *quantifiedLinearSearchAttempt) closeLocals() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.terms != nil {
		if err := a.terms.close(); err != nil {
			return err
		}
		a.terms = nil
	}
	if !a.pushed {
		return nil
	}
	level, err := a.solver.ScopeLevel()
	if err != nil {
		return err
	}
	switch level {
	case a.target:
		// Push did not publish, or Pop consumed before reporting an error.
		a.pushed = false
		return nil
	case a.target + 1:
		if err = a.solver.Pop(1); err != nil {
			return err
		}
		a.pushed = false
		return nil
	default:
		return ast.ErrASTPending
	}
}

func (a *quantifiedLinearSearchAttempt) closeModel() error {
	if a.winning == nil {
		return nil
	}
	if a.modelConsumed {
		if err := a.winning.Close(); err != nil {
			return err
		}
	} else {
		consumed, err := a.winning.DecRefConsumed()
		a.modelConsumed = consumed
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	a.winning = nil
	return nil
}

func (a *quantifiedLinearSearchAttempt) close() error {
	if err := a.closeLocals(); err != nil {
		return err
	}
	return a.closeModel()
}

type quantifiedLinearSearchFailure struct {
	cause, cleanupErr, exposed error
	message                    string
	attempt                    *quantifiedLinearSearchAttempt
}

func (f *quantifiedLinearSearchFailure) Error() string        { return f.message }
func (f *quantifiedLinearSearchFailure) Unwrap() error        { return f.exposed }
func (f *quantifiedLinearSearchFailure) OriginalCause() error { return f.cause }
func (f *quantifiedLinearSearchFailure) CleanupCause() error  { return f.cleanupErr }
func (f *quantifiedLinearSearchFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *quantifiedLinearSearchFailure) Cleanup() error {
	f.cleanupErr = f.attempt.close()
	return f.cleanupErr
}

func finishQuantifiedLinearSearch(a *quantifiedLinearSearchAttempt, cause error) (*model.ModelCore, error) {
	if cause != nil {
		errors.As(cause, &a.nested)
	}
	cleanup := a.closeLocals()
	if cause == nil && cleanup == nil {
		winning := a.winning
		a.winning = nil
		return winning, nil
	}
	if cause == nil {
		cause = cleanup
	}
	if cleanup == nil {
		cleanup = a.closeModel()
	}
	if cleanup == nil {
		return nil, cause
	}
	exposed := errors.Join(cause, cleanup)
	return nil, &quantifiedLinearSearchFailure{
		cause: cause, cleanupErr: cleanup, exposed: exposed,
		message: exposed.Error(), attempt: a,
	}
}

var _ memory.ConstructionFailure = (*quantifiedLinearSearchFailure)(nil)
