package numeral

import "errors"

// ScopedBinaryExpressionCleanupError keeps a stopped source temporary and an
// already constructed return value reachable until owning cleanup succeeds.
type ScopedBinaryExpressionCleanupError struct {
	Cause    error
	local    *ScopedBinary
	returned *ScopedBinary
}

func (e *ScopedBinaryExpressionCleanupError) Error() string { return e.Cause.Error() }
func (e *ScopedBinaryExpressionCleanupError) Unwrap() error { return e.Cause }
func (e *ScopedBinaryExpressionCleanupError) Cleanup() error {
	if e == nil {
		return nil
	}
	if e.local != nil {
		if err := e.local.Cleanup(); err != nil {
			return err
		}
		e.local = nil
	}
	if e.returned != nil {
		if err := e.returned.Cleanup(); err != nil {
			return err
		}
		e.returned = nil
	}
	return nil
}

func finishScopedBinaryExpressionSource(result **ScopedBinary, cause *error, local *ScopedBinary) {
	if local == nil {
		return
	}
	if err := local.Close(); err != nil {
		*cause = errors.Join(*cause, &ScopedBinaryExpressionCleanupError{Cause: err, local: local, returned: *result})
		*result = nil
	}
}
