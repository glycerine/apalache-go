package spacer

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
)

type invariantCheckAttempt struct {
	manager    *ast.ASTManager
	references []ast.ASTRef
	solver     *smtcore.SMTSolver
}

func (a *invariantCheckAttempt) close() error {
	// Scalar guards detect native AST admission refusal before entering the
	// solver's composite destructor. Advance only consumed reference slots.
	for len(a.references) > 0 {
		i := len(a.references) - 1
		err := a.manager.DelRef(&a.references[i])
		if a.references[i] == (ast.ASTRef{}) {
			a.references = a.references[:i]
		}
		if err != nil {
			return err
		}
	}
	if a.solver != nil {
		if err := a.solver.Close(); err != nil {
			return err
		}
		a.solver = nil
	}
	return nil
}

type invariantCheckFailure struct {
	attempt           *invariantCheckAttempt
	cause, cleanupErr error
}

func (f *invariantCheckFailure) Error() string        { return f.cause.Error() }
func (f *invariantCheckFailure) Unwrap() error        { return f.cause }
func (f *invariantCheckFailure) OriginalCause() error { return f.cause }
func (f *invariantCheckFailure) CleanupCause() error  { return f.cleanupErr }
func (f *invariantCheckFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *invariantCheckFailure) Cleanup() error {
	f.cleanupErr = f.attempt.close()
	return f.cleanupErr
}
func (a *invariantCheckAttempt) finish(cause error) error {
	cleanup := a.close()
	if cleanup == nil {
		return cause
	}
	return &invariantCheckFailure{attempt: a, cause: errors.Join(cause, cleanup), cleanupErr: cleanup}
}

var _ memory.ConstructionFailure = (*invariantCheckFailure)(nil)
