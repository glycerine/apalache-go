package smtcore

// Source: src/smt/asserted_formulas.cpp:146-171, the expr_ref/proof_ref locals
// that span rewriting and insertion into the persistent assertion stack.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// Destroy the rewritten expression before its proof, as the source locals
// require, then release the original-input guards that span cache flushing.
// Keep the remaining roots when a native release refuses or has consumed its
// root before reporting an error.
type smtAssertionScope struct {
	manager   *ast.ASTManager
	inputs    [2]ast.ASTRef
	rewritten [2]ast.ASTHandle
	nested    memory.ConstructionFailure
}

func (s *smtAssertionScope) close() error {
	if s.nested != nil {
		if err := s.nested.Cleanup(); err != nil {
			return err
		}
		s.nested = nil
	}
	for i := 0; i < len(s.rewritten); i++ {
		if s.rewritten[i].IsNull() {
			continue
		}
		consumed, err := s.manager.DecRefConsumed(s.rewritten[i])
		if consumed {
			s.rewritten[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	for i := len(s.inputs) - 1; i >= 0; i-- {
		if s.inputs[i] != (ast.ASTRef{}) {
			if err := s.manager.DelRef(&s.inputs[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

type smtAssertionCleanupFailure struct {
	scope                    *smtAssertionScope
	runtime                  *memory.Runtime
	cause, cleanup, reported error
	message                  string
}

func (f *smtAssertionCleanupFailure) Error() string                 { return f.message }
func (f *smtAssertionCleanupFailure) Unwrap() error                 { return f.reported }
func (f *smtAssertionCleanupFailure) OriginalCause() error          { return f.cause }
func (f *smtAssertionCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *smtAssertionCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *smtAssertionCleanupFailure) Cleanup() error {
	f.cleanup = f.scope.close()
	return f.cleanup
}

func (s *smtAssertionScope) finish(cause error) error {
	if cause != nil {
		errors.As(cause, &s.nested)
	}
	if cleanup := s.close(); cleanup != nil {
		reported := errors.Join(cause, cleanup)
		return &smtAssertionCleanupFailure{
			scope: s, runtime: s.manager.NumeralService().RuntimeOwner(),
			cause: cause, cleanup: cleanup, reported: reported, message: reported.Error(),
		}
	}
	return cause
}

var _ memory.ConstructionFailure = (*smtAssertionCleanupFailure)(nil)
