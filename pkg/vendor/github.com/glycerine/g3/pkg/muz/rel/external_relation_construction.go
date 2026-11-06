package rel

// Source: src/muz/rel/dl_external_relation.cpp constructor/clone/complement
// expression owners and src/api/api_datalog.cpp callback failure transfer.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalRelationConstructionOwner struct {
	manager *ast.ASTManager
	temps   []ast.ASTRef
	result  *ExternalRelation
}

type externalRelationConstructionFailure struct {
	cause      error
	cleanupErr error
	reported   error
	reportText string
	child      memory.ConstructionFailure
	owner      *externalRelationConstructionOwner
}

var _ memory.ConstructionFailure = (*externalRelationConstructionFailure)(nil)

func (f *externalRelationConstructionFailure) Error() string        { return f.reportText }
func (f *externalRelationConstructionFailure) Unwrap() error        { return f.reported }
func (f *externalRelationConstructionFailure) OriginalCause() error { return f.cause }
func (f *externalRelationConstructionFailure) CleanupCause() error  { return f.cleanupErr }
func (f *externalRelationConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *externalRelationConstructionFailure) Cleanup() error {
	if f.child != nil {
		if f.cleanupErr = f.child.Cleanup(); f.cleanupErr != nil {
			return f.cleanupErr
		}
		f.child = nil
	}
	f.cleanupErr = f.owner.closeAll()
	return f.cleanupErr
}

func (o *externalRelationConstructionOwner) closeTemps() error {
	for i := len(o.temps) - 1; i >= 0; i-- {
		if o.temps[i] == (ast.ASTRef{}) {
			continue
		}
		if err := o.manager.DelRef(&o.temps[i]); err != nil {
			return err
		}
	}
	return nil
}

func (o *externalRelationConstructionOwner) closeResult() error {
	if o.result == nil {
		return nil
	}
	if err := o.result.Close(); err != nil {
		return err
	}
	o.result = nil
	return nil
}

func (o *externalRelationConstructionOwner) closeAll() error {
	if err := o.closeTemps(); err != nil {
		return err
	}
	return o.closeResult()
}

// dispose publishes result only when the originating operation and temporary
// finalization both succeed. A refused finalizer transfers every remaining
// concrete owner and any nested construction receipt to one retryable error.
func (o *externalRelationConstructionOwner) dispose(cause error) error {
	cleanup := o.closeTemps()
	if cleanup == nil && cause != nil {
		cleanup = o.closeResult()
	}
	if cleanup == nil {
		return cause
	}
	reported := cleanup
	failure := &externalRelationConstructionFailure{cause: cause, cleanupErr: cleanup, reported: cleanup, owner: o}
	if cause == nil {
		failure.cause = cleanup
	} else {
		reported = errors.Join(cause, cleanup)
		errors.As(cause, &failure.child)
	}
	failure.reported = reported
	failure.reportText = reported.Error()
	return failure
}
