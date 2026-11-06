package sat

// Source: src/sat/sat_solver.cpp:3435-3465; src/util/util.h:176-186.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *Solver) emitConsequenceBackjump() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return out.AppendCString("(sat.get-consequences backjump)\n")
}

func (s *Solver) emitConsequenceProgress(iterations uint32, remaining *satMUSLiteralSet, outRows *satSolverLiteralRows, status lbool.LBool, preAssigned uint32, lits satConsequenceInput) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	remainingCount := remaining.size()
	fixed, err := outRows.rowCount()
	if err != nil {
		return err
	}
	total, err := lits.size()
	if err != nil {
		return err
	}
	unfixed := total - fixed - remainingCount // source unsigned arithmetic
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString("(sat.get-consequences iterations: "); err != nil {
		return err
	}
	if err = out.AppendUint32(iterations); err != nil {
		return err
	}
	if err = out.AppendCString(" variables: "); err != nil {
		return err
	}
	if err = out.AppendUint32(remainingCount); err != nil {
		return err
	}
	if err = out.AppendCString(" fixed: "); err != nil {
		return err
	}
	if err = out.AppendUint32(fixed); err != nil {
		return err
	}
	if err = out.AppendCString(" status: " + status.String() + " pre-assigned: "); err != nil {
		return err
	}
	if err = out.AppendUint32(preAssigned); err != nil {
		return err
	}
	if err = out.AppendCString(" unfixed: "); err != nil {
		return err
	}
	if err = out.AppendUint32(unfixed); err != nil {
		return err
	}
	return out.AppendCString(")\n")
}
