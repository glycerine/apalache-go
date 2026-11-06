package memory

// Source: src/util/debug.h:63; src/util/small_object_allocator.cpp:72-145,198.

// AllocatorAssertionControl is one cold borrowed actual assertion owner. No
// function closure, hot-record dispatch, or mirrored assertion state is owned.
type AllocatorAssertionControl interface{ AssertionsEnabled() (bool, error) }

func (a *SmallAllocator) sourceAssert(condition bool) error {
	if !a.config.Profile.Debug {
		return nil
	}
	enabled := true
	if a.config.Assertions != nil {
		var e error
		enabled, e = a.config.Assertions.AssertionsEnabled()
		if e != nil {
			return e
		}
	}
	if enabled && !condition {
		return ErrAllocatorAssertion
	}
	return nil
}
