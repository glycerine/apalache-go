package smt2

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// fixedpointAnswerCleanupFailure keeps a query answer rooted when printing a
// fixedpoint result is followed by a refused AST release.
type fixedpointAnswerCleanupFailure struct {
	manager    *ast.ASTManager
	reference  ast.ASTRef
	cause      error
	cleanupErr error
}

var _ memory.ConstructionFailure = (*fixedpointAnswerCleanupFailure)(nil)

func (f *fixedpointAnswerCleanupFailure) Error() string {
	if err := errors.Join(f.cause, f.cleanupErr); err != nil {
		return err.Error()
	}
	return "smt2: fixedpoint answer cleanup remains pending"
}

func (f *fixedpointAnswerCleanupFailure) Unwrap() error {
	return errors.Join(f.cause, f.cleanupErr)
}

func (f *fixedpointAnswerCleanupFailure) Cleanup() error {
	if f == nil || f.manager == nil || f.reference == (ast.ASTRef{}) {
		return nil
	}
	f.cleanupErr = f.manager.DelRef(&f.reference)
	if f.reference == (ast.ASTRef{}) {
		f.manager = nil
	}
	return f.cleanupErr
}

func (f *fixedpointAnswerCleanupFailure) RuntimeOwner() *memory.Runtime {
	if f == nil || f.manager == nil || f.manager.NumeralService() == nil {
		return nil
	}
	return f.manager.NumeralService().RuntimeOwner()
}

func (f *fixedpointAnswerCleanupFailure) OriginalCause() error { return f.cause }
func (f *fixedpointAnswerCleanupFailure) CleanupCause() error  { return f.cleanupErr }
