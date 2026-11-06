package fp

// Source: pdr_dl_interface.cpp scoped model_ref used by the empty-program
// model diagnostic. Go cleanup refusal must preserve that temporary owner.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

type pdrModelPresentation struct {
	manager *ast.ASTManager
	model   *model.Model
}

type pdrModelPresentationCleanupFailure struct {
	cause, cleanupErr error
	owner             *pdrModelPresentation
}

func (f *pdrModelPresentationCleanupFailure) Error() string        { return f.cause.Error() }
func (f *pdrModelPresentationCleanupFailure) Unwrap() error        { return f.cause }
func (f *pdrModelPresentationCleanupFailure) OriginalCause() error { return f.cause }
func (f *pdrModelPresentationCleanupFailure) CleanupCause() error  { return f.cleanupErr }
func (f *pdrModelPresentationCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *pdrModelPresentationCleanupFailure) Cleanup() error {
	f.cleanupErr = f.owner.close()
	return f.cleanupErr
}
func (o *pdrModelPresentation) close() error {
	if o.model == nil {
		return nil
	}
	if err := o.model.Close(); err != nil {
		return err
	}
	o.model = nil
	return nil
}
func (o *pdrModelPresentation) finish(cause error) error {
	if cleanup := o.close(); cleanup != nil {
		return &pdrModelPresentationCleanupFailure{cause: errors.Join(cause, cleanup), cleanupErr: cleanup, owner: o}
	}
	return cause
}
