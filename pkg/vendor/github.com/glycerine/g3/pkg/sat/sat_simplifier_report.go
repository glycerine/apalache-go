package sat

// Source: src/sat/sat_simplifier.cpp:785-806,1058-1077,1438-1458.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (s *satSimplifier) reportTail(out *utext.NumericTextStream, watch *stopwatch.Stopwatch) error {
	if err := appendSATMemoryStat(out, s.lifecycle.propagation.memory); err != nil {
		return err
	}
	if err := out.AppendCString(" :time "); err != nil {
		return err
	}
	if err := out.SetFloatMode(utext.NumericFixedFloat); err != nil {
		return err
	}
	if err := out.SetPrecision(2); err != nil {
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

func (s *satSimplifier) emitSubsumeReport() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(" (sat-subsumer :subsumed "); err != nil {
		return err
	}
	if err = out.AppendUint32(s.numSubsumed - s.subReportSubsumed); err != nil {
		return err
	}
	if err = out.AppendCString(" :subsumption-resolution "); err != nil {
		return err
	}
	if err = out.AppendUint32(s.numSubRes - s.subReportSubRes); err != nil {
		return err
	}
	if err = out.AppendCString(" :threshold "); err != nil {
		return err
	}
	if err = out.AppendInt32(s.kernel.subCounter); err != nil {
		return err
	}
	return s.reportTail(out, &s.subReport)
}

func (s *satSimplifier) emitBlockedReport() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(" (sat-blocked-clauses :elim-blocked-clauses "); err != nil {
		return err
	}
	if err = out.AppendUint32(s.numBlocked - s.blockedReportBlocked); err != nil {
		return err
	}
	return s.reportTail(out, &s.blockedReport)
}

func (s *satSimplifier) emitEliminationReport() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(" (sat-resolution :elim-bool-vars "); err != nil {
		return err
	}
	if err = out.AppendUint32(s.numElimVars - s.elimReportVars); err != nil {
		return err
	}
	if err = out.AppendCString(" :threshold "); err != nil {
		return err
	}
	if err = out.AppendInt32(s.kernel.elimCounter); err != nil {
		return err
	}
	return s.reportTail(out, &s.elimReport)
}
