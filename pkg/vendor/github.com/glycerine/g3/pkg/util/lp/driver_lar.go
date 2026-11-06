package lp

// Source: src/test/lp.cpp:2416-2477, run_lar_solver/get_solution_map.

import (
	"fmt"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/util/stopwatch"
)

// LARRunDriver borrows an existing solver, optional reader and arguments. It
// owns the comparison solver, named-value snapshots and infeasibility evidence.
// Failed work remains reachable until Close; close this driver before its reader.
type LARRunDriver struct {
	Solver     *LARSolver
	Reader     *RationalMPSReader
	Arguments  *DriverArguments
	Status     Status
	comparison *RationalPrimalSimplex
	values     *IndexedRationalVector
	evidence   *SparseRationalVector
	out        io.Writer
	watch      stopwatch.Stopwatch
	started    bool
	closed     bool
}

func NewLARRunDriver(s *LARSolver, reader *RationalMPSReader, args *DriverArguments, config MPSDriverConfig) *LARRunDriver {
	if config.Output == nil {
		config.Output = os.Stdout
	}
	return &LARRunDriver{Solver: s, Reader: reader, Arguments: args, out: config.Output, watch: stopwatch.New(config.Clock)}
}

func (d *LARRunDriver) Run() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if d.Solver == nil || d.Arguments == nil {
		return ErrBounds
	}
	s, args := d.Solver, d.Arguments
	s.Settings.MessageOut, s.Settings.DebugOut = d.out, d.out
	n, present, err := args.uintValue("--maxng")
	if err != nil {
		return err
	}
	if present {
		s.Settings.MaxIterationsWithoutImprovements = n
	}
	if args.OptionIsUsed("-pd") {
		s.Settings.PresolveWithDoubleSolverForLAR = true
	}
	if n, present, err = args.uintValue("--max_iters"); err != nil {
		return err
	} else if present {
		s.Settings.MaxTotalIterations = n
	}
	if args.OptionIsUsed("--compare_with_primal") {
		if err = d.compare(); err != nil {
			return err
		}
		return d.Close()
	}
	if err = d.watch.Start(); err != nil {
		return err
	}
	d.Status, err = s.Solve()
	if err != nil {
		return err
	}
	if _, err = io.WriteString(d.out, "status is "); err != nil {
		return err
	}
	status, err := StatusString(d.Status)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(d.out, status+", processed for "); err != nil {
		return err
	}
	span, err := d.watch.Seconds()
	if err != nil {
		return err
	}
	if _, err = io.WriteString(d.out, floatLPNumberText(span)+" seconds, and "); err != nil {
		return err
	}
	if err = writeDriverLine(d.out, fmt.Sprintf("%d iterations", s.TotalIterations())); err != nil {
		return err
	}
	if s.Bounds.Status.Value == StatusInfeasible {
		d.evidence = NewSparseRationalVector(s.manager)
		if err = s.GetInfeasibilityExplanation(d.evidence); err != nil {
			return err
		}
		// The source evidence vector is local to this if-block.
		if err = d.evidence.Close(); err != nil {
			return err
		}
		d.evidence = nil
	}
	if args.OptionIsUsed("--randomize_lar") {
		if s.Bounds.Status.Value != StatusOptimal {
			if err = writeDriverLine(d.out, "cannot check randomize on an infeazible  problem"); err != nil {
				return err
			}
			return d.Close()
		}
		if err = writeDriverLine(d.out, "checking randomize"); err != nil {
			return err
		}
		vars := s.GetListOfAllVarIndices()
		if len(vars) > 100 {
			vars = vars[:100]
		}
		if err = s.RandomUpdate(vars); err != nil {
			return err
		}
	}
	return d.Close()
}

func (d *LARRunDriver) compare() error {
	if d.Reader == nil {
		return writeDriverLine(d.out, "cannot compare with primal, the reader is null ")
	}
	var err error
	d.comparison, err = d.Reader.CreatePrimalSolver()
	if err != nil {
		return err
	}
	d.comparison.Settings.MessageOut, d.comparison.Settings.DebugOut = d.out, d.out
	if err = d.comparison.FindMaximalSolution(); err != nil {
		return err
	}
	names := d.Reader.ColumnNames()
	d.values = NewIndexedRationalVector(d.Reader.manager, uint32(len(names)))
	// The source builds a local solution map and never reads it. Retain every
	// concrete named getter and owning value without implementing unused STL.
	for i, name := range names {
		if err = d.comparison.GetColumnValueByName(name, &d.values.Data[i]); err != nil {
			return err
		}
	}
	if _, err = io.WriteString(d.out, "status = "); err != nil {
		return err
	}
	d.Status = d.Solver.Bounds.Status.Value
	status, err := StatusString(d.Status)
	if err != nil {
		return err
	}
	return writeDriverLine(d.out, status)
}

func (d *LARRunDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.comparison != nil {
		if err := d.comparison.Close(); err != nil {
			return err
		}
		d.comparison = nil
	}
	if d.values != nil {
		if err := d.values.Close(); err != nil {
			return err
		}
		d.values = nil
	}
	if d.evidence != nil {
		if err := d.evidence.Close(); err != nil {
			return err
		}
		d.evidence = nil
	}
	d.closed = true
	return nil
}
