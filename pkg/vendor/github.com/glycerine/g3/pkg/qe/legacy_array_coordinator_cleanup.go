package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

// legacyArraySelectCleanupFailure retains the actual child result after the
// coordinator has copied its outputs but native cleanup refuses release.
type legacyArraySelectCleanupFailure struct {
	child          *legacyArraySelectProjection
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyArraySelectCleanupFailure)(nil)

func (f *legacyArraySelectCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *legacyArraySelectCleanupFailure) Unwrap() error                 { return f.reported }
func (f *legacyArraySelectCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyArraySelectCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyArraySelectCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyArraySelectCleanupFailure) Cleanup() error {
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

func closeTransferredLegacyArraySelect(child *legacyArraySelectProjection, cause error) error {
	if cleanup := child.Close(); cleanup != nil {
		return &legacyArraySelectCleanupFailure{
			child: child, runtime: child.manager.NumeralService().RuntimeOwner(), cause: cause, cleanup: cleanup,
			reported: errors.Join(cause, cleanup),
		}
	}
	return cause
}
