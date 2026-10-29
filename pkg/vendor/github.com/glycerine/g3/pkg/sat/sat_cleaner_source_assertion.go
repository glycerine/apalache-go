package sat

// Source: src/sat/sat_cleaner.cpp:43-57; src/util/debug.h:63-65.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func (c *satCleaner) assertionsEnabled() (bool, error) {
	if !satStorageSourceDebug {
		return false, nil
	}
	return diagnostic.BorrowProcessDebugContext().AssertionsEnabled()
}

func (c *satCleaner) sourceAssertion(ok bool, line int32, condition string) error {
	if ok {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	if !c.debugActions || !d.HasAssertionActions() {
		return ErrSATAssertion
	}
	return diagnostic.DebugAssertionFailure(d, "src/sat/sat_cleaner.cpp", line, condition)
}
