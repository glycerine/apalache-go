package sat_solver

// Source: inc_sat_solver ownership around sat::solver's debug destructor guard.

import "github.com/glycerine/g3/pkg/util/memory"

// CleanupFailure preserves an already observed SAT destructor stop. Cleanup
// explicitly acknowledges that receipt; ordinary Close never suppresses it.
type CleanupFailure struct {
	owner *Solver
	cause error
}

var _ memory.ConstructionFailure = (*CleanupFailure)(nil)

func (e *CleanupFailure) Error() string        { return e.cause.Error() }
func (e *CleanupFailure) Unwrap() error        { return e.cause }
func (e *CleanupFailure) OriginalCause() error { return e.cause }
func (e *CleanupFailure) CleanupCause() error  { return e.cause }
func (e *CleanupFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *CleanupFailure) Cleanup() error {
	s := e.owner
	if s.closed {
		return nil
	}
	if s.core != nil && s.core.StoppedCleanupCause() != nil {
		if err := s.core.CleanupStopped(); err != nil {
			return err
		}
	}
	return s.Close()
}
