package fp

// Source: pdr_dl_interface construction/updt_params ownership.

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/util/memory"
)

// pdrContextConstructionFailure keeps a partially configured context
// reachable when its native cleanup is refused. Callers can retry through the
// standard memory.ConstructionFailure contract.
type pdrContextConstructionFailure struct {
	cause      error
	runtime    *memory.Runtime
	cleanupErr error
	context    *pdr.Context
}

func (f *pdrContextConstructionFailure) Error() string {
	return errors.Join(f.cause, f.cleanupErr).Error()
}
func (f *pdrContextConstructionFailure) Unwrap() error { return errors.Join(f.cause, f.cleanupErr) }
func (f *pdrContextConstructionFailure) Cleanup() error {
	if f == nil {
		return nil
	}
	if f.context == nil {
		f.cleanupErr = nil
		return nil
	}
	f.cleanupErr = f.context.Close()
	if f.cleanupErr != nil {
		return f.cleanupErr
	}
	f.context = nil
	return nil
}

func (f *pdrContextConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *pdrContextConstructionFailure) OriginalCause() error          { return f.cause }
func (f *pdrContextConstructionFailure) CleanupCause() error           { return f.cleanupErr }

func failPDRContextConstruction(context *pdr.Context, runtime *memory.Runtime, cause error) (*pdr.Context, error) {
	if context == nil {
		return nil, cause
	}
	cleanup := context.Close()
	if cleanup == nil {
		return nil, cause
	}
	return nil, &pdrContextConstructionFailure{cause: cause, runtime: runtime, cleanupErr: cleanup, context: context}
}
