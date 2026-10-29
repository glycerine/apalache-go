package lp

// Source: src/test/lp.cpp:3080-3266, retained test_lp_local/tst_lp execution.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

// LPTestDriver executes the source-selected action. It borrows the numeral
// manager and shared random generator. Completed child metadata remains
// inspectable; a failed child stays owned until Close. Close this owner before
// its borrowed manager. Owning drivers must not be copied.
type LPTestDriver struct {
	Preparation       *LPDriverPreparation
	ExitCode          uint32
	Finished          bool
	RationalBenchmark *RationalBenchmarkDriver
	PairBenchmark     *PairBenchmarkDriver
	BoundPropagation  *BoundPropagationDriver
	LUCheck           *LUCheckFileDriver
	Permutations      *PermutationTestDriver
	Stacked           *StackedTestDriver
	Example           *LARExampleDriver
	Zero              *LPZeroDriver
	File              *MPSFileDriver
	FileList          *MPSFileListDriver
	Batch             *MPSBatchDriver
	lpDriverDebug
	manager         *numeral.RationalManager
	random          *z3rand.Generator
	config          MPSDriverConfig
	started, closed bool
}

func NewLPTestDriver(manager *numeral.RationalManager, random *z3rand.Generator, config MPSDriverConfig) *LPTestDriver {
	config.Output = driverReportOutput(config.Output)
	return &LPTestDriver{manager: manager, random: random, config: config}
}

// RunTestEntry applies tst_lp's literal argv+2/argc-2 slice. The source's
// referenced test index is unused. A too-short Go slice returns ErrBounds.
func (d *LPTestDriver) RunTestEntry(argv []string) error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	if len(argv) < 2 {
		d.started = true
		return ErrBounds
	}
	return d.Run(argv[2:])
}

// Run takes the local LP argument slice, including any free program-name
// argument supplied by the caller. There is no additional argv skip here.
func (d *LPTestDriver) Run(argv []string) error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	var err error
	d.Preparation, err = PrepareLPDriver(argv, d.config.Output)
	d.ExitCode = d.Preparation.ExitCode
	if err != nil {
		return err
	}
	p := d.Preparation
	switch p.Action {
	case "": // ordinary return or parse failure; finalize itself is empty
	case "test_rationals":
		d.PairBenchmark = NewPairBenchmarkDriver(d.manager, d.config)
		err = d.PairBenchmark.Run(d.random)
	case "test_rationals_no_numeric_pairs", "test_rationals_no_numeric_pairs_plus":
		d.RationalBenchmark = NewRationalBenchmarkDriver(d.manager, p.Action == "test_rationals_no_numeric_pairs_plus", d.config)
		err = d.RationalBenchmark.Run(d.random)
	case "test_int_set":
		err = RunDriverIntSetTest(d.config.Output)
	case "test_bound_propagation":
		d.BoundPropagation = NewBoundPropagationDriver(d.manager, d.config.Output)
		err = d.BoundPropagation.Run()
	case "check_lu_from_file":
		d.LUCheck = NewLUCheckFileDriver(p.File, d.config.Output)
		err = d.LUCheck.Run()
	case "test_permutations":
		d.Permutations = &PermutationTestDriver{}
		err = d.Permutations.Run(d.config.Output)
	case "test_files_from_directory": // retained source body is commented out
	case "filelist":
		d.FileList = NewMPSFileListDriver(p.File, p.Arguments, d.manager, d.config)
		err = d.FileList.Run()
	case "test_binary_priority_queue":
		err = RunDriverPriorityQueueTest(d.config.Output, d.random)
	case "test_evidence_for_total_inf_simple", "test_term":
		d.Example = NewLARExampleDriver(d.manager, d.config.Output)
		if p.Action == "test_term" {
			err = d.Example.RunTerm()
		} else {
			err = d.Example.RunEvidence()
		}
	case "test_lp_0":
		d.Zero = NewLPZeroDriver(d.config.Output)
		err = d.Zero.Run()
	case "test_stacked":
		d.Stacked = &StackedTestDriver{}
		err = d.Stacked.Run(d.config.Output)
	case "solve_mps":
		d.File = NewMPSFileDriver(p.File, p.Arguments, d.manager, d.config)
		err = d.File.RunWithOptions(MPSDriverSolveOptions{Minimum: p.Minimum, Exact: p.Exact, Dual: p.Dual, CompareWithPrimal: p.CompareWithPrimal, MaxIterations: p.MaxIterations, TimeLimit: p.TimeLimit})
	case "solve_some_mps":
		err = d.runBatch()
	default:
		err = d.runDebug()
	}
	if err != nil {
		return err
	}
	d.Finished = true
	return d.Close()
}

func (d *LPTestDriver) Close() error {
	if d.closed {
		return nil
	}
	if err := d.closeDebug(); err != nil {
		return err
	}
	if d.Batch != nil {
		if err := d.Batch.Close(); err != nil {
			return err
		}
	}
	if d.File != nil {
		if err := d.File.Close(); err != nil {
			return err
		}
	}
	if d.FileList != nil {
		if err := d.FileList.Close(); err != nil {
			return err
		}
	}
	if d.Zero != nil {
		if err := d.Zero.Close(); err != nil {
			return err
		}
	}
	if d.Example != nil {
		if err := d.Example.Close(); err != nil {
			return err
		}
	}
	if d.LUCheck != nil {
		if err := d.LUCheck.Close(); err != nil {
			return err
		}
	}
	if d.BoundPropagation != nil {
		if err := d.BoundPropagation.Close(); err != nil {
			return err
		}
	}
	if d.PairBenchmark != nil {
		if err := d.PairBenchmark.Close(); err != nil {
			return err
		}
	}
	if d.RationalBenchmark != nil {
		if err := d.RationalBenchmark.Close(); err != nil {
			return err
		}
	}
	d.closed = true
	return nil
}
