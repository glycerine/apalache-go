package sat

// Source: src/sat/sat_cleaner.cpp:184,200; src/util/debug.h:64.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func (c *satCleaner) assertInvariant() error {
	if !satStorageSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := d.ConditionalAssertionEnabled("cleaner_bug")
	if err != nil || !enabled {
		return err
	}
	// check_invariant() returns true after its nested SASSERT(checker()).
	// Other nested checker refusals remain explicit until their source
	// assertions are represented; the outer condition itself cannot be false.
	return c.lifecycle.checkInvariantWithActions(true)
}
