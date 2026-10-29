package sat

// Source: src/sat/sat_solver.cpp:2071-2084; src/util/util.h:176-186.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func (s *satSolverConflictResolution) emitCoreUpdateNotice(minSize, coreSize uint32) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString("(sat.updating core "); err != nil {
		return err
	}
	if err = out.AppendUint32(minSize); err != nil {
		return err
	}
	if err = out.AppendCString(" "); err != nil {
		return err
	}
	if err = out.AppendUint32(coreSize); err != nil {
		return err
	}
	return out.AppendCString(")\n")
}

func (s *satSolverConflictResolution) emitCoreReport() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(2)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString("(sat.core: "); err != nil {
		return err
	}
	if err = satMUSWriteCore(out, &s.core); err != nil {
		return err
	}
	return out.AppendCString(")\n")
}
