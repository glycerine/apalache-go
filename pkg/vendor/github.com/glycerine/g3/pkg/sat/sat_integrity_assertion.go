package sat

// Source: src/sat/sat_integrity_checker.cpp:133-189; src/util/debug.h:60-65.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func satIntegrityAssertion(ok, actions bool, line int32, condition string) error {
	if ok {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	if !actions || !d.HasAssertionActions() {
		return ErrSATAssertion
	}
	return diagnostic.DebugAssertionFailure(d, "src/sat/sat_integrity_checker.cpp", line, condition)
}
