package sat

// Source: src/sat/sat_asymm_branch.cpp:35-52; src/sat/sat_types.h:31.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (a *satAsymmBranch) emitReport() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(" (sat-asymm-branch :elim-literals "); err != nil {
		return err
	}
	if err = out.AppendUint32(a.elimLiterals - a.reportElimLiterals); err != nil {
		return err
	}
	if err = out.AppendCString(" :cost "); err != nil {
		return err
	}
	if err = out.AppendInt32(a.lifecycle.propagation.asymmCounter); err != nil {
		return err
	}
	if err = appendSATMemoryStat(out, a.lifecycle.propagation.memory); err != nil {
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
	seconds, err := a.report.GetSeconds()
	if err != nil {
		return err
	}
	if err = out.AppendFloat64(seconds); err != nil {
		return err
	}
	return out.AppendCString(")\n")
}
