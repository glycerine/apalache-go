package numeral

// Source: src/util/scoped_numeral.h:160-183; concrete scoped_mpff operator
// return-value copy followed by the source temporary's destructor.

import "errors"

// ScopedFastFloatExpressionCleanupError retains the stopped source temporary,
// its already constructed result or a failed result copy constructor.
type ScopedFastFloatExpressionCleanupError struct {
	Cause    error
	local    *ScopedFastFloat
	returned *ScopedFastFloat
	copy     *ScopedFastFloatCopyCleanupError
}

func (e *ScopedFastFloatExpressionCleanupError) Error() string { return e.Cause.Error() }
func (e *ScopedFastFloatExpressionCleanupError) Unwrap() error { return e.Cause }
func (e *ScopedFastFloatExpressionCleanupError) Cleanup() error {
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
	if e.copy != nil {
		if err := e.copy.Cleanup(); err != nil {
			return err
		}
		e.copy = nil
	}
	return nil
}

func finishScopedFastFloatExpression(local *ScopedFastFloat, result **ScopedFastFloat, cause *error) {
	if local == nil {
		return
	}
	if end := local.Close(); end != nil {
		var copy *ScopedFastFloatCopyCleanupError
		errors.As(*cause, &copy)
		*cause = &ScopedFastFloatExpressionCleanupError{
			Cause: errors.Join(*cause, end), local: local, returned: *result, copy: copy,
		}
		*result = nil
	}
}
