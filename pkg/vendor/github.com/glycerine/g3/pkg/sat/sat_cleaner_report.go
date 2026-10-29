package sat

// Source: src/sat/sat_cleaner.cpp:158-179; src/sat/sat_types.h:31.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (c *satCleaner) emitReport(watch *stopwatch.Stopwatch, startClauses, startLiterals uint32) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(" (sat-cleaner :elim-literals "); err != nil {
		return err
	}
	if err = out.AppendUint32(c.elimLiterals - startLiterals); err != nil {
		return err
	}
	if err = out.AppendCString(" :elim-clauses "); err != nil {
		return err
	}
	if err = out.AppendUint32(c.elimClauses - startClauses); err != nil {
		return err
	}
	if err = out.AppendCString(" :cost "); err != nil {
		return err
	}
	if err = out.AppendInt32(c.lifecycle.propagation.cleanerCounter); err != nil {
		return err
	}
	if err = c.solver.appendCompactStat(out); err != nil {
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
