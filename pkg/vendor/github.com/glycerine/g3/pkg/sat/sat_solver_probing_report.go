package sat

// Source: src/sat/sat_probing.cpp:171-188; src/sat/sat_types.h:289-293.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (s *satSolverProbing) emitReport(watch *stopwatch.Stopwatch, oldAssigned uint32) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(" (sat-probing :probing-assigned "); err != nil {
		return err
	}
	if err = out.AppendUint32(s.numAssigned - oldAssigned); err != nil {
		return err
	}
	if err = out.AppendCString(" :cost "); err != nil {
		return err
	}
	if err = out.AppendInt32(s.propagation().probingCounter); err != nil {
		return err
	}
	if s.stoppedAt != 0 {
		if err = out.AppendCString(" :stopped-at "); err != nil {
			return err
		}
		if err = out.AppendUint32(s.stoppedAt); err != nil {
			return err
		}
	}
	if err = appendSATMemoryStat(out, s.propagation().memory); err != nil {
		return err
	}
	if err = out.AppendCString(" :time "); err != nil {
		return err
	}
	if err = out.SetFloatMode(utext.NumericFixedFloat); err != nil {
		return err
	}
	if err = out.SetPrecision(2); err != nil {
		return err
	}
	seconds, err := watch.GetSeconds()
	if err != nil {
		return err
	}
	if err = out.AppendFloat64(seconds); err != nil {
		return err
	}
	return out.AppendCString(")\n")
}
