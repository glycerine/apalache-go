package lp

// Source: src/test/lp.cpp:2477-2506,2529-2545.

import (
	"os"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// LARFileDriver owns the two readers and LAR solver of test_lar_on_file.
// The selected creation reader dies before the second read on success;
// failed population retains it until the partial solver has closed.
// Arguments and the manager are borrowed. The second read always uses MPS,
// including when --smt selected the creation reader, as in the source.
type LARFileDriver struct {
	Name       string
	Arguments  *DriverArguments
	Solver     *LARSolver
	Runner     *LARRunDriver
	Status     Status
	Iterations uint32
	creator    *RationalMPSReader
	smtCreator *SMTReader
	reader     *RationalMPSReader
	manager    *numeral.RationalManager
	config     MPSDriverConfig
	started    bool
	closed     bool
}

func NewLARFileDriver(name string, args *DriverArguments, manager *numeral.RationalManager, config MPSDriverConfig) *LARFileDriver {
	if config.Output == nil {
		config.Output = os.Stdout
	}
	return &LARFileDriver{Name: name, Arguments: args, manager: manager, config: config}
}

func (d *LARFileDriver) Run() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if d.Arguments == nil {
		return ErrBounds
	}
	var err error
	if d.Arguments.OptionIsUsed("--smt") {
		err = d.createSMT()
	} else {
		err = d.createMPS()
	}
	if err != nil {
		return err
	}
	d.reader = OpenRationalMPSReader(d.manager, d.Name)
	d.reader.Messages = d.config.Output
	if err := d.reader.Read(); err != nil {
		return err
	}
	if d.reader.IsOK() {
		d.Runner = NewLARRunDriver(d.Solver, d.reader, d.Arguments, d.config)
		if err := d.Runner.Run(); err != nil {
			return err
		}
		d.Status, d.Iterations = d.Runner.Status, d.Solver.TotalIterations()
	}
	return d.Close()
}

func (d *LARFileDriver) createMPS() error {
	d.creator = OpenRationalMPSReader(d.manager, d.Name)
	d.creator.Messages = d.config.Output
	if err := d.creator.Read(); err != nil {
		return err
	}
	if d.creator.IsOK() {
		var err error
		d.Solver, err = d.creator.CreateLARSolver()
		if err != nil {
			return err
		}
	} else if err := writeDriverLine(d.config.Output, "cannot process "+d.Name); err != nil {
		return err
	}
	// Successful population has no remaining borrowed reader numerals. Source
	// create_lar_solver_from_file destroys this local reader before returning.
	if err := d.creator.Close(); err != nil {
		return err
	}
	d.creator = nil
	return nil
}

func (d *LARFileDriver) createSMT() error {
	d.smtCreator = OpenSMTReader(d.manager, d.Name)
	d.smtCreator.Messages = d.config.Output
	if err := d.smtCreator.Read(); err != nil {
		return err
	}
	if d.smtCreator.IsOK() {
		var err error
		d.Solver, err = d.smtCreator.CreateLARSolver()
		if err != nil {
			return err
		}
	} else if err := writeDriverLine(d.config.Output, "cannot process "+d.Name); err != nil {
		return err
	}
	if err := d.smtCreator.Close(); err != nil {
		return err
	}
	d.smtCreator = nil
	return nil
}

func (d *LARFileDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.Runner != nil {
		if err := d.Runner.Close(); err != nil {
			return err
		}
		d.Runner = nil
	}
	if d.Solver != nil {
		if err := d.Solver.Close(); err != nil {
			return err
		}
		d.Solver = nil
	}
	if d.creator != nil {
		if err := d.creator.Close(); err != nil {
			return err
		}
		d.creator = nil
	}
	if d.smtCreator != nil {
		if err := d.smtCreator.Close(); err != nil {
			return err
		}
		d.smtCreator = nil
	}
	if d.reader != nil {
		if err := d.reader.Close(); err != nil {
			return err
		}
		d.reader = nil
	}
	d.closed = true
	return nil
}

// LARInputDriver implements test_lar_solver's file-before-list dispatch for
// MPS or SMT creation. Completed files close before the next starts. A failed file remains
// Current until Close; owning drivers must not be copied.
type LARInputDriver struct {
	Arguments *DriverArguments
	Files     []string
	Completed int
	Current   *LARFileDriver
	manager   *numeral.RationalManager
	config    MPSDriverConfig
	started   bool
	closed    bool
}

func NewLARInputDriver(args *DriverArguments, manager *numeral.RationalManager, config MPSDriverConfig) *LARInputDriver {
	if config.Output == nil {
		config.Output = os.Stdout
	}
	return &LARInputDriver{Arguments: args, manager: manager, config: config}
}

func (d *LARInputDriver) Run() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if d.Arguments == nil {
		return ErrBounds
	}
	if file := d.Arguments.OptionValue("--file"); file != "" {
		d.Files = []string{file}
	} else if list := d.Arguments.OptionValue("--filelist"); list != "" {
		var err error
		d.Files, err = ReadDriverFileList(list, d.config.Output)
		if err != nil {
			return err
		}
	}
	for _, file := range d.Files {
		d.Current = NewLARFileDriver(file, d.Arguments, d.manager, d.config)
		if err := d.Current.Run(); err != nil {
			return err
		}
		d.Current = nil
		d.Completed++
	}
	return d.Close()
}

func (d *LARInputDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.Current != nil {
		if err := d.Current.Close(); err != nil {
			return err
		}
		d.Current = nil
	}
	d.closed = true
	return nil
}
