package numeral

// Source: src/util/scoped_numeral.h:31-33. A failed copy constructor does
// not invoke _scoped_numeral's destructor.

// ScopedIntCopyCleanupError retains a partially constructed scoped integer.
type ScopedIntCopyCleanupError struct {
	Cause error
	owner *ScopedInt
}

func (e *ScopedIntCopyCleanupError) Error() string { return e.Cause.Error() }
func (e *ScopedIntCopyCleanupError) Unwrap() error { return e.Cause }
func (e *ScopedIntCopyCleanupError) Cleanup() error {
	if e == nil || e.owner == nil {
		return nil
	}
	if err := e.owner.Cleanup(); err != nil {
		return err
	}
	e.owner = nil
	return nil
}

// ScopedRationalCopyCleanupError retains a partially constructed rational.
type ScopedRationalCopyCleanupError struct {
	Cause error
	owner *ScopedRational
}

func (e *ScopedRationalCopyCleanupError) Error() string { return e.Cause.Error() }
func (e *ScopedRationalCopyCleanupError) Unwrap() error { return e.Cause }
func (e *ScopedRationalCopyCleanupError) Cleanup() error {
	if e == nil || e.owner == nil {
		return nil
	}
	if err := e.owner.Cleanup(); err != nil {
		return err
	}
	e.owner = nil
	return nil
}
