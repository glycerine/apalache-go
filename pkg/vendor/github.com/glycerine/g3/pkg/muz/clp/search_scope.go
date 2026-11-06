package clp

// Source: src/muz/clp/clp_context.cpp imp::search per-rule solver scope.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

// clpSolverScope owns one rule-attempt scope. Push may publish before it
// reports failure and Pop may consume before doing the same, so cleanup uses
// the observed depth instead of assuming either operation was atomic.
type clpSolverScope struct {
	solver Solver
	target uint32
	pushed bool
}

func (s *clpSolverScope) begin() error {
	var err error
	s.target, err = s.solver.ScopeLevel()
	if err != nil {
		return err
	}
	s.pushed = true
	return s.solver.Push()
}

func (s *clpSolverScope) close() error {
	if !s.pushed {
		return nil
	}
	level, err := s.solver.ScopeLevel()
	if err != nil {
		return err
	}
	switch level {
	case s.target:
		// Pop consumed the scope before reporting failure, or Push never
		// published it.
		s.pushed = false
		return nil
	case s.target + 1:
		if err = s.solver.Pop(1); err != nil {
			return err
		}
		s.pushed = false
		return nil
	default:
		return ast.ErrASTPending
	}
}

// clpScopedRuleAttempt keeps an inner failed search ahead of its parent scope,
// and keeps every grounded assertion alive until the solver has released that
// scope.
type clpScopedRuleAttempt struct {
	manager *ast.ASTManager
	scope   clpSolverScope
	nested  memory.ConstructionFailure
	rule    *ruleAttemptOwner
}

func (a *clpScopedRuleAttempt) close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if err := a.scope.close(); err != nil {
		return err
	}
	if a.rule != nil {
		if err := a.rule.close(); err != nil {
			return err
		}
		a.rule = nil
	}
	return nil
}

type clpScopeCleanupFailure struct {
	cause, cleanupErr, exposed error
	message                    string
	attempt                    *clpScopedRuleAttempt
}

func (f *clpScopeCleanupFailure) Error() string        { return f.message }
func (f *clpScopeCleanupFailure) Unwrap() error        { return f.exposed }
func (f *clpScopeCleanupFailure) OriginalCause() error { return f.cause }
func (f *clpScopeCleanupFailure) CleanupCause() error  { return f.cleanupErr }
func (f *clpScopeCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *clpScopeCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.close()
	return f.cleanupErr
}

func finishCLPScopedRule(attempt *clpScopedRuleAttempt, cause error) error {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			attempt.nested = nested
		}
	}
	cleanup := attempt.close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
	}
	exposed := errors.Join(cause, cleanup)
	return &clpScopeCleanupFailure{
		cause: cause, cleanupErr: cleanup, exposed: exposed,
		message: exposed.Error(), attempt: attempt,
	}
}

func (e *Engine) tryScopedRule(rule *base.Rule, goals []ast.ASTHandle, depth uint32, index int) (candidate lbool.LBool, resultErr error) {
	attempt := &clpScopedRuleAttempt{
		manager: e.manager,
		scope:   clpSolverScope{solver: e.solver},
		rule: &ruleAttemptOwner{
			manager:     e.manager,
			environment: &groundEnvironment{manager: e.manager},
		},
	}
	defer func() {
		resultErr = finishCLPScopedRule(attempt, resultErr)
		if resultErr != nil {
			candidate = lbool.Undef
		}
	}()
	if err := attempt.scope.begin(); err != nil {
		return lbool.Undef, err
	}
	return e.tryRule(rule, goals, depth, index, attempt.rule)
}

var _ memory.ConstructionFailure = (*clpScopeCleanupFailure)(nil)
