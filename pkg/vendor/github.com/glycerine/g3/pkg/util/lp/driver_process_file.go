package lp

// Source: src/test/lp.cpp:2116-2130,2265-2309.

import (
	"errors"
	"os"
	"os/exec"
	"strconv"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func DriverLPOutputName(minimum bool, name string, exact bool) string {
	direction := "max"
	if minimum {
		direction = "min"
	}
	result := name + "_lp_tst_" + direction
	if exact {
		return result + "_mpq.out"
	}
	return result + ".out"
}
func DriverGLPKOutputName(minimum bool, name string) string {
	if minimum {
		return name + "_min_glpk_out"
	}
	return name + "_max_glpk_out"
}

// RunDriverGLPK executes the retained fixed glpsol command using native argv.
// Literal filenames remain single arguments; shell tokenization is not ported.
// Nonzero exit and launch failure both reach the caller's inconclusive branch.
// A launch error is retained separately from an ordinary process exit code.
func RunDriverGLPK(name, report string, minimum bool, timeLimit uint32) (int, error) {
	direction := "--max"
	if minimum {
		direction = "--min"
	}
	args := []string{"--nointopt", "--nomip", direction}
	if timeLimit > 0 {
		args = append(args, "--tmlim", strconv.FormatUint(uint64(timeLimit), 10))
	}
	args = append(args, "-o", report, name)
	command := exec.Command("glpsol", args...)
	command.Stderr = os.Stderr // source redirects stdout only; nil Stdout discards it
	err := command.Run()
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return -1, err
}

// ProcessFileDriver owns the report file and the concrete child of one
// process_test_file call. Arguments, counts and numerical manager are borrowed.
// Counts retain source publication order. Failed children remain owned until
// cleanup succeeds, including when their original failure was caught by source.
type ProcessFileDriver struct {
	InputDirectory, Name, OutputDirectory string
	Arguments                             *DriverArguments
	Counts                                *DriverComparisonCounts
	ReportName, GLPKReportName            string
	CaughtError, GLPKLaunchError          error
	GLPKExitCode                          int
	MPS                                   *MPSFileDriver
	LAR                                   *LARFileDriver
	file                                  *os.File
	manager                               *numeral.RationalManager
	config                                MPSDriverConfig
	maxIterations, timeLimit              uint32
	started, closed                       bool
}

func NewProcessFileDriver(directory, name string, args *DriverArguments, outDirectory string, maxIterations, timeLimit uint32, counts *DriverComparisonCounts, manager *numeral.RationalManager, config MPSDriverConfig) *ProcessFileDriver {
	config.Output = driverReportOutput(config.Output)
	return &ProcessFileDriver{InputDirectory: directory, Name: name, Arguments: args, OutputDirectory: outDirectory, Counts: counts, manager: manager, config: config, maxIterations: maxIterations, timeLimit: timeLimit}
}

func (d *ProcessFileDriver) Run() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if d.Arguments == nil || d.Counts == nil {
		return ErrBounds
	}
	exact, minimum := d.Arguments.OptionIsUsed("--mpq"), d.Arguments.OptionIsUsed("--min")
	d.ReportName = d.OutputDirectory + "/" + DriverLPOutputName(minimum, d.Name, exact)
	input := d.InputDirectory + "/" + d.Name
	if input[len(input)-1] == '~' {
		return d.Close()
	}
	if err := writeDriverLine(d.config.Output, "processing "+input); err != nil {
		return err
	}
	var err error
	d.file, err = os.Create(d.ReportName)
	if err != nil {
		if err = writeDriverLine(d.config.Output, "cannot open file "+d.ReportName); err != nil {
			return err
		}
		return ErrDriverReport
	}
	childConfig := d.config
	childConfig.Output = d.file
	dual := d.Arguments.OptionIsUsed("--dual")
	if d.Arguments.OptionIsUsed("--lar") {
		d.LAR = NewLARFileDriver(input, d.Arguments, d.manager, childConfig)
		err = d.LAR.Run()
	} else {
		d.MPS = NewMPSFileDriver(input, d.Arguments, d.manager, childConfig)
		err = d.MPS.RunWithOptions(MPSDriverSolveOptions{Minimum: minimum, Exact: exact, Dual: dual, MaxIterations: d.maxIterations, TimeLimit: d.timeLimit})
	}
	if err != nil {
		d.CaughtError = err
		if err = writeDriverLine(d.file, "catching the failure"); err != nil {
			return err
		}
		d.Counts.Failures++
		return d.Close()
	}
	if d.Arguments.OptionIsUsed("--compare_with_glpk") {
		d.GLPKReportName = d.OutputDirectory + "/" + DriverGLPKOutputName(minimum, d.Name)
		d.GLPKExitCode, d.GLPKLaunchError = RunDriverGLPK(input, d.GLPKReportName, minimum, d.timeLimit)
		if d.GLPKExitCode != 0 {
			if err = writeDriverLine(d.config.Output, "glpk failed"); err != nil {
				return err
			}
			d.Counts.Inconclusives++
		} else if err = CompareGLPKDriverReports(d.GLPKReportName, d.ReportName, d.Counts, d.config.Output); err != nil {
			return err
		}
	}
	return d.Close()
}

func (d *ProcessFileDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.MPS != nil {
		if err := d.MPS.Close(); err != nil {
			return err
		}
		d.MPS = nil
	}
	if d.LAR != nil {
		if err := d.LAR.Close(); err != nil {
			return err
		}
		d.LAR = nil
	}
	if d.file != nil {
		if err := d.file.Close(); err != nil {
			return err
		}
		d.file = nil
	}
	d.closed = true
	return nil
}
