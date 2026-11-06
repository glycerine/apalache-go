package fp

// Source: model_converter application to the owning model_ref returned by
// PDR/Spacer. Preserve that model when native failure cleanup is refused.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

type hornModelConversionOwner struct {
	manager *ast.ASTManager
	model   *model.Model
}
type hornModelConversionCleanupFailure struct {
	cause, cleanupErr error
	owner             *hornModelConversionOwner
}

func (f *hornModelConversionCleanupFailure) Error() string        { return f.cause.Error() }
func (f *hornModelConversionCleanupFailure) Unwrap() error        { return f.cause }
func (f *hornModelConversionCleanupFailure) OriginalCause() error { return f.cause }
func (f *hornModelConversionCleanupFailure) CleanupCause() error  { return f.cleanupErr }
func (f *hornModelConversionCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *hornModelConversionCleanupFailure) Cleanup() error {
	f.cleanupErr = f.owner.close()
	return f.cleanupErr
}
func (o *hornModelConversionOwner) close() error {
	if o.model == nil {
		return nil
	}
	if err := o.model.Close(); err != nil {
		return err
	}
	o.model = nil
	return nil
}
func (o *hornModelConversionOwner) fail(cause error) error {
	if cleanup := o.close(); cleanup != nil {
		return &hornModelConversionCleanupFailure{cause: errors.Join(cause, cleanup), cleanupErr: cleanup, owner: o}
	}
	return cause
}
