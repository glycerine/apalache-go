package clp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type clpRefOwner struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
	nested  memory.ConstructionFailure
}

type clpRefCleanupFailure struct {
	cause, cleanupErr, exposed error
	message                    string
	owner                      *clpRefOwner
}

func (f *clpRefCleanupFailure) Error() string { return f.message }
func (f *clpRefCleanupFailure) Unwrap() error { return f.exposed }
func (f *clpRefCleanupFailure) Cleanup() error {
	f.cleanupErr = f.owner.close()
	return f.cleanupErr
}
func (f *clpRefCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *clpRefCleanupFailure) OriginalCause() error { return f.cause }
func (f *clpRefCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (o *clpRefOwner) take(ref ast.ASTRef) {
	o.refs = append(o.refs, ref)
}

func (o *clpRefOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	for i := len(o.refs) - 1; i >= 0; i-- {
		if o.refs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := o.manager.DelRef(&o.refs[i]); err != nil {
			return err
		}
	}
	o.refs = nil
	return nil
}

func (o *clpRefOwner) dispose() error {
	return o.finish(nil)
}

func (o *clpRefOwner) finish(cause error) error {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			o.nested = nested
		}
	}
	cleanup := o.close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
	}
	exposed := errors.Join(cause, cleanup)
	return &clpRefCleanupFailure{
		cause: cause, cleanupErr: cleanup, exposed: exposed,
		message: exposed.Error(), owner: o,
	}
}

var _ memory.ConstructionFailure = (*clpRefCleanupFailure)(nil)
