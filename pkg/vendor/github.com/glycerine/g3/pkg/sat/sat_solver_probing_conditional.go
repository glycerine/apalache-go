package sat

// Source: src/sat/sat_probing.cpp:200,241; src/util/debug.h:64.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func (s *satSolverProbing) assertConditionalInvariant() error {
	if !satStorageSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := d.ConditionalAssertionEnabled("probing")
	if err != nil || !enabled {
		return err
	}
	// The source checker consumes its resource limit and performs nested
	// SASSERT actions before returning to this outer CASSERT.
	return s.lifecycle.checkInvariantWithActions(true)
}
