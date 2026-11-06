package sat

// Source: src/sat/sat_solver.cpp:1208-1230; src/util/debug.h:64.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func (s *Solver) checkSimplificationDiagnostics() error {
	if err := s.checkSimplificationMissedPropagation(); err != nil {
		return err
	}
	return s.checkSimplificationInvariant()
}

func (s *Solver) checkSimplificationMissedPropagation() error {
	if !satStorageSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := d.ConditionalAssertionEnabled("sat_missed_prop")
	if err != nil {
		return err
	}
	if enabled {
		return s.Lifecycle.checkMissedPropagation()
	}
	return nil
}

func (s *Solver) checkSimplificationInvariant() error {
	if !satStorageSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := d.ConditionalAssertionEnabled("sat_simplify_bug")
	if err != nil {
		return err
	}
	if enabled {
		return s.Lifecycle.checkInvariantWithActions(true)
	}
	return nil
}
