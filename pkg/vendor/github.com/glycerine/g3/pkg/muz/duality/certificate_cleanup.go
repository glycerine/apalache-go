package duality

// Source: dl_interface::display_certificate_non_const owns the safe-query
// model_ref until printing and destruction finish. A refused Go destructor
// carries that same snapshot forward for cleanup instead of losing its owner.

import (
	"errors"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

type certificateModelScope struct {
	snapshot *model.Model
	runtime  *memory.Runtime
	nested   memory.ConstructionFailure
}

func (s *certificateModelScope) cleanup() error {
	if s.nested != nil {
		if err := s.nested.Cleanup(); err != nil {
			return err
		}
		s.nested = nil
	}
	if s.snapshot != nil {
		if err := s.snapshot.Close(); err != nil {
			return err
		}
		s.snapshot = nil
	}
	return nil
}

func (s *certificateModelScope) finish(cause error) error {
	errors.As(cause, &s.nested)
	cleanup := s.cleanup()
	if cleanup == nil {
		return cause
	}
	reported := errors.Join(cause, cleanup)
	return &certificateModelFailure{scope: s, original: cause, cleanup: cleanup, reported: reported, message: reported.Error()}
}

type certificateModelFailure struct {
	scope                       *certificateModelScope
	original, cleanup, reported error
	message                     string
}

func (f *certificateModelFailure) Error() string                 { return f.message }
func (f *certificateModelFailure) Unwrap() error                 { return f.reported }
func (f *certificateModelFailure) OriginalCause() error          { return f.original }
func (f *certificateModelFailure) CleanupCause() error           { return f.cleanup }
func (f *certificateModelFailure) RuntimeOwner() *memory.Runtime { return f.scope.runtime }
func (f *certificateModelFailure) Cleanup() error {
	f.cleanup = f.scope.cleanup()
	return f.cleanup
}

var _ memory.ConstructionFailure = (*certificateModelFailure)(nil)
