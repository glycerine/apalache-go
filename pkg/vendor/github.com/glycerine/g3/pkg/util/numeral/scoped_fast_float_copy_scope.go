package numeral

// Source: src/util/scoped_numeral.h:31-33; concrete mpff_manager copy.

// ScopedFastFloatCopyCleanupError retains a failed copy constructor's owner.
type ScopedFastFloatCopyCleanupError struct {
	Cause error
	owner *ScopedFastFloat
}

func (e *ScopedFastFloatCopyCleanupError) Error() string { return e.Cause.Error() }
func (e *ScopedFastFloatCopyCleanupError) Unwrap() error { return e.Cause }
func (e *ScopedFastFloatCopyCleanupError) Cleanup() error {
	if e == nil || e.owner == nil {
		return nil
	}
	if err := e.owner.Cleanup(); err != nil {
		return err
	}
	e.owner = nil
	return nil
}
