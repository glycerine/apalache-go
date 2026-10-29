package sat

// Source: src/sat/sat_solver.cpp:1460-1512,1518-1607.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func (g *satSolverGC) emitHalfReport(strategy satGCStrategy, deleted uint32) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	name := ""
	switch strategy {
	case satGCGlue:
		name = "glue"
	case satGCPSM:
		name = "psm"
	case satGCGluePSM:
		name = "glue-psm"
	case satGCPSMGlue:
		name = "psm-glue"
	default:
		return ErrSATAssertion
	}
	if err = out.AppendCString("(sat-gc :strategy "); err != nil {
		return err
	}
	if err = out.AppendCString(name); err != nil {
		return err
	}
	if err = out.AppendCString(" :deleted "); err != nil {
		return err
	}
	if err = out.AppendUint32(deleted); err != nil {
		return err
	}
	return out.AppendCString(")\n")
}

func (g *satSolverGC) emitDynamicReport(d float64, frozen, activated, deleted uint32) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString("(sat-gc :d_tk "); err != nil {
		return err
	}
	if err = out.AppendFloat64(d); err != nil {
		return err
	}
	if err = out.AppendCString(" :min-d_tk "); err != nil {
		return err
	}
	if err = out.AppendFloat64(g.minDTk); err != nil {
		return err
	}
	if err = out.AppendCString(" :frozen "); err != nil {
		return err
	}
	if err = out.AppendUint32(frozen); err != nil {
		return err
	}
	if err = out.AppendCString(" :activated "); err != nil {
		return err
	}
	if err = out.AppendUint32(activated); err != nil {
		return err
	}
	if err = out.AppendCString(" :deleted "); err != nil {
		return err
	}
	if err = out.AppendUint32(deleted); err != nil {
		return err
	}
	return out.AppendCString(")\n")
}
