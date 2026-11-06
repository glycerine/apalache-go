package qe

// Source: spacer_qe_project.cpp array_project_selects_util::project local result.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyArraySelectConstructionFailure struct {
	owner          *legacyArraySelectProjection
	nested         memory.ConstructionFailure
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
	text           string
}

var _ memory.ConstructionFailure = (*legacyArraySelectConstructionFailure)(nil)

func (f *legacyArraySelectConstructionFailure) Error() string                 { return f.text }
func (f *legacyArraySelectConstructionFailure) Unwrap() error                 { return f.reported }
func (f *legacyArraySelectConstructionFailure) OriginalCause() error          { return f.cause }
func (f *legacyArraySelectConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyArraySelectConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyArraySelectConstructionFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.owner == nil || f.owner.manager == nil {
		f.owner = nil
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.owner.Close()
	if f.cleanup == nil {
		f.owner = nil
	}
	return f.cleanup
}

func closeFailedLegacyArraySelectProjection(owner *legacyArraySelectProjection, cause error) error {
	cleanup := owner.Close()
	if cleanup == nil {
		return cause
	}
	origin := cause
	if origin == nil {
		origin = cleanup
	}
	reported := cleanup
	if cause != nil {
		reported = errors.Join(cause, cleanup)
	}
	var nested memory.ConstructionFailure
	_ = errors.As(cause, &nested)
	return &legacyArraySelectConstructionFailure{
		owner: owner, nested: nested, runtime: owner.manager.NumeralService().RuntimeOwner(),
		cause: origin, cleanup: cleanup, reported: reported, text: reported.Error(),
	}
}
