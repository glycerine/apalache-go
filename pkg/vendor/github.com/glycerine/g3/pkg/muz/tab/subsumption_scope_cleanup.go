package tab

// Source: src/muz/tab/tab_context.cpp, index::is_subsumed and
// index::check_substitution solver-scope ownership.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/memory"
)

// subsumptionSolverScope owns one solver scope. Push may publish the scope
// before returning an error, and Pop may consume it before doing the same, so
// cleanup always reconciles the observed depth with the saved source depth.
type subsumptionSolverScope struct {
	solver solver.Solver
	target uint32
	pushed bool
}

func (s *subsumptionSolverScope) begin() error {
	var err error
	s.target, err = s.solver.ScopeLevel()
	if err != nil {
		return err
	}
	// Push can expose a new scope and still report cleanup refusal.
	s.pushed = true
	return s.solver.Push()
}

func (s *subsumptionSolverScope) Close() error {
	if !s.pushed {
		return nil
	}
	level, err := s.solver.ScopeLevel()
	if err != nil {
		return err
	}
	switch level {
	case s.target:
		// Pop consumed the scope before returning an error.
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

// subsumptionCleanupAttempt keeps the scope ahead of every owner used to
// construct its assertions. An outer attempt may adopt an inner validation
// receipt; it must drain that receipt before inspecting its own solver depth.
type subsumptionCleanupAttempt struct {
	manager       *ast.ASTManager
	scope         subsumptionSolverScope
	nested        memory.ConstructionFailure
	post          ast.ASTRef
	forms         []ast.ASTRef
	search        *subsumptionSearch
	falseLiterals map[ast.ASTHandle]ast.ASTRef
	bindings      *substitution.Substitution
	grounded      *groundClause
}

func (a *subsumptionCleanupAttempt) Close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if err := a.scope.Close(); err != nil {
		return err
	}
	if a.post != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.post); err != nil {
			return err
		}
	}
	for len(a.forms) != 0 {
		index := len(a.forms) - 1
		err := a.manager.DelRef(&a.forms[index])
		if a.forms[index] == (ast.ASTRef{}) {
			a.forms = a.forms[:index]
		}
		if err != nil {
			return err
		}
		if len(a.forms) != index {
			return ast.ErrASTPending
		}
	}
	if a.search != nil {
		if err := a.search.truncateConditions(0); err != nil {
			return err
		}
		a.search = nil
	}
	for key, owned := range a.falseLiterals {
		err := a.manager.DelRef(&owned)
		if owned == (ast.ASTRef{}) {
			delete(a.falseLiterals, key)
		} else {
			a.falseLiterals[key] = owned
		}
		if err != nil {
			return err
		}
	}
	if len(a.falseLiterals) == 0 {
		a.falseLiterals = nil
	}
	if a.bindings != nil {
		if err := a.bindings.Close(); err != nil {
			return err
		}
		a.bindings = nil
	}
	if a.grounded != nil {
		if err := a.grounded.Close(); err != nil {
			return err
		}
		a.grounded = nil
	}
	return nil
}

type subsumptionScopeFailure struct {
	cause, cleanupErr, exposed error
	message                    string
	attempt                    *subsumptionCleanupAttempt
}

func (f *subsumptionScopeFailure) Error() string { return f.message }
func (f *subsumptionScopeFailure) Unwrap() error { return f.exposed }
func (f *subsumptionScopeFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *subsumptionScopeFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *subsumptionScopeFailure) OriginalCause() error { return f.cause }
func (f *subsumptionScopeFailure) CleanupCause() error  { return f.cleanupErr }

func finishSubsumptionScope(attempt *subsumptionCleanupAttempt, cause error) error {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			attempt.nested = nested
		}
	}
	cleanup := attempt.Close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
	}
	exposed := errors.Join(cause, cleanup)
	return &subsumptionScopeFailure{
		cause: cause, cleanupErr: cleanup, exposed: exposed,
		message: exposed.Error(), attempt: attempt,
	}
}

var _ memory.ConstructionFailure = (*subsumptionScopeFailure)(nil)
