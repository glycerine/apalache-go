package sat

// Source: src/sat/sat_solver.cpp:1678; src/util/debug.h:64.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSolverConflictResolution) assertNoMarks() error {
	if !satStorageSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := d.ConditionalAssertionEnabled("sat_check_marks")
	if err != nil || !enabled {
		return err
	}
	v := s.analysis.propagation.variables
	n, err := v.numVarsChecked()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		marked, e := s.analysis.marked(i)
		if e != nil {
			return e
		}
		if marked {
			// CASSERT calls check_marks(), whose loop asserts each marked
			// variable before it can return true to the outer macro.
			if e = diagnostic.DebugAssertionFailure(d, "src/sat/sat_solver.cpp", 2854, "!is_marked(v)"); e != nil {
				return e
			}
		}
	}
	return nil
}
