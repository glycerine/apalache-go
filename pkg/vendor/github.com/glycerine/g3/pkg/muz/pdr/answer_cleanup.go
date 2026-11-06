package pdr

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type pdrAnswerExportOwner struct {
	manager     *ast.ASTManager
	temporaries []ast.ASTRef
	result      ast.ASTRef
}

type pdrAnswerExportFailure struct {
	cause      error
	cleanupErr error
	owner      *pdrAnswerExportOwner
	runtime    *memory.Runtime
}

func (f *pdrAnswerExportFailure) Error() string { return errors.Join(f.cause, f.cleanupErr).Error() }
func (f *pdrAnswerExportFailure) Unwrap() error { return errors.Join(f.cause, f.cleanupErr) }
func (f *pdrAnswerExportFailure) Cleanup() error {
	if f == nil || f.owner == nil {
		return nil
	}
	f.cleanupErr = f.owner.close()
	if f.cleanupErr == nil {
		f.owner = nil
	}
	return f.cleanupErr
}
func (f *pdrAnswerExportFailure) RuntimeOwner() *memory.Runtime {
	if f == nil {
		return nil
	}
	return f.runtime
}
func (f *pdrAnswerExportFailure) OriginalCause() error { return f.cause }
func (f *pdrAnswerExportFailure) CleanupCause() error  { return f.cleanupErr }

func (o *pdrAnswerExportOwner) closeResult() error {
	if o.result == (ast.ASTRef{}) {
		return nil
	}
	return o.manager.DelRef(&o.result)
}

func (o *pdrAnswerExportOwner) closeTemporaries() error {
	for len(o.temporaries) != 0 {
		index := len(o.temporaries) - 1
		if o.temporaries[index] != (ast.ASTRef{}) {
			if err := o.manager.DelRef(&o.temporaries[index]); err != nil {
				return err
			}
		}
		o.temporaries = o.temporaries[:index]
	}
	return nil
}

func (o *pdrAnswerExportOwner) close() error {
	if err := o.closeResult(); err != nil {
		return err
	}
	return o.closeTemporaries()
}

func (o *pdrAnswerExportOwner) finish(result *ast.ASTRef, resultErr *error) {
	cleanup := o.closeTemporaries()
	if cleanup == nil && *resultErr == nil {
		return
	}
	if *result != (ast.ASTRef{}) {
		o.result = *result
		*result = ast.ASTRef{}
	}
	if cleanup == nil {
		cleanup = o.closeResult()
	}
	if cleanup == nil {
		return
	}
	cause := *resultErr
	if cause == nil {
		cause = fmt.Errorf("pdr: close answer constraints: %w", cleanup)
	}
	*resultErr = &pdrAnswerExportFailure{cause: cause, cleanupErr: cleanup, owner: o, runtime: o.manager.NumeralService().RuntimeOwner()}
}
