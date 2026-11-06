package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

// Cover export owns both the incoming formula and every model used to rebuild
// the original predicate. Cleanup refusal cannot transfer a partial result.
type hornCoverAttempt struct {
	manager        *ast.ASTManager
	source, result ast.ASTRef
	seed, expanded *model.Model
	interpretation *model.FunctionInterpretation
}
type hornCoverCleanupFailure struct {
	cause, cleanupErr error
	attempt           *hornCoverAttempt
}

func (f *hornCoverCleanupFailure) Error() string        { return f.cause.Error() }
func (f *hornCoverCleanupFailure) Unwrap() error        { return f.cause }
func (f *hornCoverCleanupFailure) OriginalCause() error { return f.cause }
func (f *hornCoverCleanupFailure) CleanupCause() error  { return f.cleanupErr }
func (f *hornCoverCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *hornCoverCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.close()
	return f.cleanupErr
}
func (a *hornCoverAttempt) closeTemporary() error {
	if a.interpretation != nil {
		if err := a.interpretation.Close(); err != nil {
			return err
		}
		a.interpretation = nil
	}
	if a.expanded != nil {
		if err := a.expanded.Close(); err != nil {
			return err
		}
		if a.seed == a.expanded {
			a.seed = nil
		}
		a.expanded = nil
	}
	if a.seed != nil {
		if err := a.seed.Close(); err != nil {
			return err
		}
		a.seed = nil
	}
	if a.source != (ast.ASTRef{}) {
		return a.manager.DelRef(&a.source)
	}
	return nil
}
func (a *hornCoverAttempt) close() error {
	if err := a.closeTemporary(); err != nil {
		return err
	}
	if a.result != (ast.ASTRef{}) {
		return a.manager.DelRef(&a.result)
	}
	return nil
}
func (a *hornCoverAttempt) finish(cause error) (ast.ASTRef, error) {
	cause = errors.Join(cause, a.closeTemporary())
	if cause == nil {
		result := a.result
		a.result = ast.ASTRef{}
		return result, nil
	}
	if cleanup := a.close(); cleanup != nil {
		return ast.ASTRef{}, &hornCoverCleanupFailure{cause: errors.Join(cause, cleanup), cleanupErr: cleanup, attempt: a}
	}
	return ast.ASTRef{}, cause
}
