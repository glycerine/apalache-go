package sat

// Source: src/sat/sat_scc.cpp:43-58; src/sat/sat_types.h:31.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (c *satSCC) emitReport() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(" (sat-scc :elim-vars "); err != nil {
		return err
	}
	if err = out.AppendUint32(c.numElim - c.reportNumElim); err != nil {
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
	seconds, err := c.report.GetSeconds()
	if err != nil {
		return err
	}
	if err = out.AppendFloat64(seconds); err != nil {
		return err
	}
	return out.AppendCString(")\n")
}
