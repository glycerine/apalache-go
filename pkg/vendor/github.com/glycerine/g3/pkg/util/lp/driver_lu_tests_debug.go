//go:build g3debug

package lp

// Source: src/test/lp.cpp:503-508, retained test_lu parent sequence.

import (
	"io"

	z3rand "github.com/glycerine/g3/pkg/util/random"
)

// LUTestDriver runs each child to its source return boundary before starting
// the next. Closed child metadata records the completed prefix; on failure the
// current child's lexical owners remain reachable until Close.
type LUTestDriver struct {
	Small              *SmallLUTestDriver
	Larger, Randomized *LargerLUTestDriver
	Holes              *HolesLUTestDriver
	CompletedScenarios uint32
	ExitCode           int
	settings           *Settings
	out                io.Writer
	started, closed    bool
}

func NewLUTestDriver(settings *Settings, out io.Writer) *LUTestDriver {
	return &LUTestDriver{settings: settings, out: driverReportOutput(out)}
}

func (d *LUTestDriver) Run(random *z3rand.Generator) error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	d.Small = NewSmallLUTestDriver(d.settings, d.out)
	if err := d.Small.Run(); err != nil {
		return err
	}
	d.CompletedScenarios++
	d.Larger = NewLargerLUTestDriver(d.settings, d.out, false)
	if err := d.Larger.Run(random); err != nil {
		d.ExitCode = d.Larger.ExitCode
		return err
	}
	d.CompletedScenarios++
	d.Holes = NewHolesLUTestDriver(d.settings, d.out)
	if err := d.Holes.Run(); err != nil {
		return err
	}
	d.CompletedScenarios++
	d.Randomized = NewLargerLUTestDriver(d.settings, d.out, true)
	if err := d.Randomized.Run(random); err != nil {
		return err
	}
	d.CompletedScenarios++
	return d.Close()
}

func (d *LUTestDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.Randomized != nil {
		if err := d.Randomized.Close(); err != nil {
			return err
		}
	}
	if d.Holes != nil {
		if err := d.Holes.Close(); err != nil {
			return err
		}
	}
	if d.Larger != nil {
		if err := d.Larger.Close(); err != nil {
			return err
		}
	}
	if d.Small != nil {
		if err := d.Small.Close(); err != nil {
			return err
		}
	}
	d.closed = true
	return nil
}
