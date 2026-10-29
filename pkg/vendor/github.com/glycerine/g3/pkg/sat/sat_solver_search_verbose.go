package sat

// Source: src/sat/sat_solver.cpp:723-790,1191-1196,1276-1280,1347-1355.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (s *Solver) emitCheckNotice() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(2)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return out.AppendCString("(sat.sat-solver)\n")
}

func (s *Solver) emitSearchAbort(reason string, conflicts bool) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString("(sat \"abort: " + reason); err != nil {
		return err
	}
	if conflicts {
		if err = out.AppendUint32(s.conflict.conflicts); err != nil {
			return err
		}
	}
	return out.AppendCString("\")\n")
}

func (s *Solver) emitModelCheckNotice() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return out.AppendCString("\"checking model\"\n")
}

func (s *Solver) emitSimplifyNotice() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(2)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return out.AppendCString("(sat.simplify)\n")
}

func (g *satSolverGC) emitRestartReport() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil {
		return err
	}
	if enabled {
		s := g.conflict.solver
		if s == nil {
			return ErrSATOwner
		}
		out, err := diagnostic.BorrowProcessVerboseStream()
		if err != nil {
			return err
		}
		if err = out.AppendCString("(sat-restart :conflicts "); err != nil {
			return err
		}
		if err = out.AppendUint32(s.conflict.conflicts); err != nil {
			return err
		}
		if err = out.AppendCString(" :decisions "); err != nil {
			return err
		}
		if err = out.AppendUint32(s.decision.decisions); err != nil {
			return err
		}
		if err = out.AppendCString(" :restarts "); err != nil {
			return err
		}
		if err = out.AppendUint32(g.restartStat); err != nil {
			return err
		}
		if err = s.appendCompactStat(out); err != nil {
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
		seconds, err := g.watch.GetCurrentSeconds()
		if err != nil {
			return err
		}
		if err = out.AppendFloat64(seconds); err != nil {
			return err
		}
		if err = out.AppendCString(")\n"); err != nil {
			return err
		}
	}
	enabled, err = diagnostic.ProcessVerboseEnabled(30)
	if err != nil || !enabled {
		return err
	}
	s := g.conflict.solver
	if s == nil {
		return ErrSATOwner
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return s.displayStatus(out)
}
