package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyRealProjectionCleanupFailure struct {
	child          *legacyRealProjection
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyRealProjectionCleanupFailure)(nil)

func (f *legacyRealProjectionCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *legacyRealProjectionCleanupFailure) Unwrap() error                 { return f.reported }
func (f *legacyRealProjectionCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyRealProjectionCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyRealProjectionCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyRealProjectionCleanupFailure) Cleanup() error {
	if f.child == nil {
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.child.Close()
	if f.cleanup == nil {
		f.child = nil
	}
	return f.cleanup
}

func closeTransferredLegacyRealProjection(child *legacyRealProjection, cause error) error {
	if cleanup := child.Close(); cleanup != nil {
		return &legacyRealProjectionCleanupFailure{
			child: child, runtime: child.manager.NumeralService().RuntimeOwner(), cause: cause, cleanup: cleanup,
			reported: errors.Join(cause, cleanup),
		}
	}
	return cause
}
