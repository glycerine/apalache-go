package lp

// Source: src/test/lp.cpp:1151-1277,2219-2227. Retained MPS driver consumers.

import (
	"io"
	"math"
	"os"

	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

// MPSDriverConfig selects native report/clock providers. Nil Output uses stdout;
// nil Clock uses the stopwatch's actual process CPU clock. Clock returns ns.
type MPSDriverConfig struct {
	Output io.Writer
	Clock  func() (uint64, error)
}

// MPSFileDriver owns the actual readers, concrete solvers and lexical exact
// objective of one retained solve_mps call. Arguments and manager are borrowed.
// Run closes successful owners at the source return boundary. On failure call
// Close before closing the numeral manager; partial children may borrow driver
// or reader values. Run cannot be retried, and owning drivers must not be copied.
type MPSFileDriver struct {
	Name            string
	Arguments       *DriverArguments
	Status          Status
	TotalIterations uint32
	floatReader     *FloatMPSReader
	rationalReader  *RationalMPSReader
	floatPrimal     *FloatPrimalSimplex
	floatDual       *FloatDualSimplex
	floatComparison *FloatPrimalSimplex
	rationalPrimal  *RationalPrimalSimplex
	rationalDual    *RationalDualSimplex
	manager         *numeral.RationalManager
	work            *IndexedRationalVector
	calls           exactEtaCalls
	watch           stopwatch.Stopwatch
	out             io.Writer
	minimum, dual   bool
	started, closed bool
	maxIterations   uint32
	timeLimit       uint32
}

func NewMPSFileDriver(name string, args *DriverArguments, manager *numeral.RationalManager, config MPSDriverConfig) *MPSFileDriver {
	out := config.Output
	if out == nil {
		out = os.Stdout
	}
	return &MPSFileDriver{Name: name, Arguments: args, manager: manager, out: out, watch: stopwatch.New(config.Clock)}
}

func (d *MPSFileDriver) Run() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if d.Arguments == nil {
		return ErrBounds
	}
	d.minimum, d.dual = d.Arguments.OptionIsUsed("--min"), d.Arguments.OptionIsUsed("--dual")
	exact := d.Arguments.OptionIsUsed("--mpq")
	compare := d.Arguments.OptionIsUsed("--compare_with_primal")
	var err error
	d.timeLimit, d.maxIterations, err = d.Arguments.TimeAndIterationLimits()
	if err != nil {
		return err
	}
	return d.runConfigured(exact, compare)
}

// MPSDriverSolveOptions is the concrete explicit-argument solve_mps overload.
// Callers retain their own snapshots of flags and limits; Arguments remains
// borrowed for setup settings read later during the actual solve.
type MPSDriverSolveOptions struct {
	Minimum, Exact, Dual, CompareWithPrimal bool
	MaxIterations, TimeLimit                uint32
}

func (d *MPSFileDriver) RunWithOptions(options MPSDriverSolveOptions) error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if d.Arguments == nil {
		return ErrBounds
	}
	d.minimum, d.dual = options.Minimum, options.Dual
	d.maxIterations, d.timeLimit = options.MaxIterations, options.TimeLimit
	return d.runConfigured(options.Exact, options.CompareWithPrimal)
}

func (d *MPSFileDriver) runConfigured(exact, compare bool) error {
	var err error
	headline := "solving " + d.Name
	if exact {
		headline += " in rationals "
	}
	if err = writeDriverLine(d.out, headline); err != nil {
		return err
	}
	if exact {
		err = d.runRational()
	} else {
		err = d.runFloat(compare)
	}
	if err != nil {
		return err
	}
	return d.Close()
}

// DriverValuesOnePercentClose preserves std::max's first operand on unordered
// comparisons, the strict tiny-magnitude bypass and the literal <= bound.
func DriverValuesOnePercentClose(a, b float64) bool {
	maximum := math.Abs(a)
	if other := math.Abs(b); maximum < other {
		maximum = other
	}
	if maximum < 0.000001 {
		return true
	}
	return math.Abs(a-b) <= maximum/100
}

func (d *MPSFileDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.manager != nil {
		if err := d.calls.cleanupRaw(d.manager); err != nil {
			return err
		}
	}
	if d.rationalPrimal != nil {
		if err := d.rationalPrimal.Close(); err != nil {
			return err
		}
		d.rationalPrimal = nil
	}
	if d.rationalDual != nil {
		if err := d.rationalDual.Close(); err != nil {
			return err
		}
		d.rationalDual = nil
	}
	if d.floatComparison != nil {
		if err := d.floatComparison.Close(); err != nil {
			return err
		}
		d.floatComparison = nil
	}
	if d.floatPrimal != nil {
		if err := d.floatPrimal.Close(); err != nil {
			return err
		}
		d.floatPrimal = nil
	}
	if d.floatDual != nil {
		if err := d.floatDual.Close(); err != nil {
			return err
		}
		d.floatDual = nil
	}
	if d.rationalReader != nil {
		if err := d.rationalReader.Close(); err != nil {
			return err
		}
		d.rationalReader = nil
	}
	if d.floatReader != nil {
		if err := d.floatReader.Close(); err != nil {
			return err
		}
		d.floatReader = nil
	}
	if d.work != nil {
		if err := d.work.Close(); err != nil {
			return err
		}
		d.work = nil
	}
	if d.manager != nil {
		if err := d.calls.cleanupRoots(d.manager); err != nil {
			return err
		}
	}
	d.closed = true
	return nil
}
