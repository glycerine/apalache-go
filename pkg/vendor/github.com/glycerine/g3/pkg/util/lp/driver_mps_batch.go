package lp

// Source: src/test/lp.cpp:1479-1724. Retained solve_some_mps orchestration.

import (
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// FillDriverMPSCatalogue binds the concrete result of getenv(HOME). A missing
// environment entry emits the source diagnostic without a newline or flush.
// Source minimum keys deliberately omit the HOME prefix used by file paths.
func FillDriverMPSCatalogue(catalogueHome string, present bool, out io.Writer) ([]string, map[string]bool, error) {
	if !present {
		_, err := io.WriteString(driverReportOutput(out), "cannot find home directory, don't know how to find the files")
		return nil, nil, err
	}
	files := make([]string, len(driverMPSRelativeFiles))
	for i, name := range driverMPSRelativeFiles {
		files[i] = catalogueHome + name
	}
	minimums := make(map[string]bool, len(driverMPSMinimumKeys))
	for _, name := range driverMPSMinimumKeys {
		minimums[name] = true
	}
	return files, minimums, nil
}

func PrepareDriverOutputDirectory(name string, out io.Writer) error {
	out = driverReportOutput(out)
	f, err := os.Open(name)
	if err == nil {
		info, e := f.Stat()
		f.Close()
		if e == nil && info.IsDir() {
			return nil
		}
	}
	if err = writeDriverLine(out, "creating directory "+name); err != nil {
		return err
	}
	if err = os.Mkdir(name, 0775); err != nil {
		return writeDriverLine(out, "Cannot open output directory \""+name+"\"")
	}
	return nil
}

func SplitDriverMPSPath(name string, out io.Writer) (string, string, error) {
	at := strings.LastIndexByte(name, '/')
	if at < 0 {
		if err := writeDriverLine(driverReportOutput(out), "cannot find file name in "+name); err != nil {
			return "", "", err
		}
		return "", "", ErrBounds // source bare throw outside a catch terminates
	}
	return name[:at], name[at+1:], nil
}

// MPSBatchDriver owns one concrete child at a time. Run uses the actual HOME
// environment; RunWithHome binds an explicit getenv result for an embedding
// caller. Failed children remain available for Close. No callback solver or
// universal template adapter is introduced. Files and Minimums are inspection
// views; callers must not mutate them during Run.
type MPSBatchDriver struct {
	Arguments                   *DriverArguments
	Files                       []string
	Minimums                    map[string]bool
	Counts                      DriverComparisonCounts
	Completed, PreludeCompleted int
	Current                     *MPSFileDriver
	Process                     *ProcessFileDriver
	manager                     *numeral.RationalManager
	config                      MPSDriverConfig
	maxIterations, timeLimit    uint32
	started, closed             bool
}

func NewMPSBatchDriver(args *DriverArguments, manager *numeral.RationalManager, config MPSDriverConfig) *MPSBatchDriver {
	config.Output = driverReportOutput(config.Output)
	return &MPSBatchDriver{Arguments: args, manager: manager, config: config}
}

func (d *MPSBatchDriver) begin() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if d.Arguments == nil {
		return ErrBounds
	}
	var err error
	d.timeLimit, d.maxIterations, err = d.Arguments.TimeAndIterationLimits()
	return err
}

func (d *MPSBatchDriver) Run() error {
	if err := d.begin(); err != nil {
		return err
	}
	catalogueHome, present := os.LookupEnv("HOME")
	return d.runHome(catalogueHome, present)
}
func (d *MPSBatchDriver) RunWithHome(catalogueHome string, present bool) error {
	if err := d.begin(); err != nil {
		return err
	}
	return d.runHome(catalogueHome, present)
}

func (d *MPSBatchDriver) runHome(catalogueHome string, present bool) error {
	var err error
	d.Files, d.Minimums, err = FillDriverMPSCatalogue(catalogueHome, present, d.config.Output)
	if err != nil {
		return err
	}
	exact := d.Arguments.OptionIsUsed("--mpq")
	dual := d.Arguments.OptionIsUsed("--dual")
	compare := d.Arguments.OptionIsUsed("--compare_with_primal")
	if d.Arguments.OptionIsUsed("--compare_with_glpk") {
		directory := d.Arguments.OptionValue("--out_dir")
		if directory == "" {
			directory = "/tmp/test"
		}
		if err = PrepareDriverOutputDirectory(directory, d.config.Output); err != nil {
			return err
		}
		for _, name := range d.Files {
			parent, base, e := SplitDriverMPSPath(name, d.config.Output)
			if e != nil {
				return e
			}
			d.Process = NewProcessFileDriver(parent, base, d.Arguments, directory, d.maxIterations, d.timeLimit, &d.Counts, d.manager, d.config)
			if err = d.Process.Run(); err != nil {
				return err
			}
			d.Process = nil
			d.Completed++
		}
		if err = d.reportCounts(); err != nil {
			return err
		}
		return d.Close()
	}
	if !exact {
		if err = d.floatPrelude(dual, compare); err != nil {
			return err
		}
	}
	// Source const-char handlers have no retained throw producer in these
	// loops. The sole live literal throw is in the known-solution prelude,
	// outside every handler; the unused mem_copy_hook has no caller.
	// std::string/integer/default exceptions and native errors propagate.
	for _, name := range d.Files {
		d.Current = NewMPSFileDriver(name, d.Arguments, d.manager, d.config)
		if exact {
			err = d.Current.RunRationalDefault(dual)
		} else {
			err = d.Current.RunWithOptions(MPSDriverSolveOptions{Minimum: d.Minimums[name], MaxIterations: d.maxIterations, TimeLimit: d.timeLimit, Dual: dual, CompareWithPrimal: compare})
		}
		if err != nil {
			return err
		}
		d.Current = nil
		d.Completed++
	}
	return d.Close()
}

func (d *MPSBatchDriver) floatPrelude(dual, compare bool) error {
	if len(d.Files) < 10 {
		return ErrBounds
	}
	explicit := func(index int) error {
		d.Current = NewMPSFileDriver(d.Files[index], d.Arguments, d.manager, d.config)
		if err := d.Current.RunWithOptions(MPSDriverSolveOptions{TimeLimit: d.timeLimit, Dual: dual, CompareWithPrimal: compare}); err != nil {
			return err
		}
		d.Current = nil
		d.PreludeCompleted++
		return nil
	}
	known := func(index int, solution map[string]float64, status Status) error {
		d.Current = NewMPSFileDriver(d.Files[index], d.Arguments, d.manager, d.config)
		if err := d.Current.RunFloatKnownSolution(solution, status, dual); err != nil {
			return err
		}
		d.Current = nil
		d.PreludeCompleted++
		return nil
	}
	if err := explicit(6); err != nil {
		return err
	}
	if err := known(3, nil, StatusInfeasible); err != nil {
		return err
	}
	solution := map[string]float64{"X1": 0, "X2": 6, "X3": 0, "X4": 15, "X5": 2, "X6": 1, "X7": 1, "X8": 0}
	if err := known(9, solution, StatusOptimal); err != nil {
		return err
	}
	if err := known(0, solution, StatusOptimal); err != nil {
		return err
	}
	clear(solution)
	solution["X1"] = 25.0 / 14.0
	if err := known(5, solution, StatusOptimal); err != nil {
		return err
	}
	if err := known(4, solution, StatusOptimal); err != nil {
		return err
	}
	if err := known(2, nil, StatusUnbounded); err != nil {
		return err
	}
	if err := known(1, nil, StatusUnbounded); err != nil {
		return err
	}
	return explicit(8)
}

func (d *MPSBatchDriver) reportCounts() error {
	out := d.config.Output
	if _, err := io.WriteString(out, "comparing with glpk: successes "); err != nil {
		return err
	}
	if _, err := io.WriteString(out, strconv.FormatUint(uint64(d.Counts.Successes), 10)); err != nil {
		return err
	}
	if _, err := io.WriteString(out, ", failures "); err != nil {
		return err
	}
	if _, err := io.WriteString(out, strconv.FormatUint(uint64(d.Counts.Failures), 10)); err != nil {
		return err
	}
	if _, err := io.WriteString(out, ", inconclusives "); err != nil {
		return err
	}
	return writeDriverLine(out, strconv.FormatUint(uint64(d.Counts.Inconclusives), 10))
}

func (d *MPSBatchDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.Current != nil {
		if err := d.Current.Close(); err != nil {
			return err
		}
		d.Current = nil
	}
	if d.Process != nil {
		if err := d.Process.Close(); err != nil {
			return err
		}
		d.Process = nil
	}
	d.closed = true
	return nil
}
