package lp

// Source: src/test/lp.cpp:1380-1419,1676-1699.

import (
	"errors"
	"io"
	"math"
	"sort"
)

// ErrDriverStatusWrong transports the source's "status is wrong" exception.
// Checked builds stop at the preceding source assertion instead.
var ErrDriverStatusWrong = errors.New("status is wrong")

// RunFloatKnownSolution runs the retained default double/double consumer.
// Arguments, clock and setup options are deliberately unused. solution is
// borrowed; each entry is copied at its iteration. Sorted Go map keys give
// diagnostics a stable order where the C++ unordered map specifies none.
// A failed solver/reader remains owned by d until Close.
func (d *MPSFileDriver) RunFloatKnownSolution(solution map[string]float64, expectedStatus Status, dual bool) error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started, d.dual = true, dual
	if err := writeDriverLine(d.out, "solving "+d.Name); err != nil {
		return err
	}
	d.floatReader = OpenFloatMPSReader(d.Name)
	d.floatReader.Messages = d.out
	if err := d.floatReader.Read(); err != nil {
		return err
	}
	if !d.floatReader.IsOK() {
		if err := writeDriverLine(d.out, "cannot process "+d.Name); err != nil {
			return err
		}
		return d.Close()
	}
	var err error
	if dual {
		d.floatDual, err = d.floatReader.CreateDualSolver()
	} else {
		d.floatPrimal, err = d.floatReader.CreatePrimalSolver()
	}
	if err != nil {
		return err
	}
	base := d.floatBase()
	base.Settings.MessageOut, base.Settings.DebugOut = d.out, d.out
	if dual {
		err = d.floatDual.FindMaximalSolution()
	} else {
		err = d.floatPrimal.FindMaximalSolution()
	}
	if err != nil {
		return err
	}
	return d.reportFloatKnownSolution(solution, expectedStatus)
}

func (d *MPSFileDriver) reportFloatKnownSolution(solution map[string]float64, expectedStatus Status) error {
	base := d.floatBase()
	var err error
	if _, err = io.WriteString(d.out, "status is "); err != nil {
		return err
	}
	d.Status, d.TotalIterations = base.Status, base.TotalIterations
	status, err := StatusString(base.Status)
	if err != nil {
		return err
	}
	if err = writeDriverLine(d.out, status); err != nil {
		return err
	}
	if expectedStatus != base.Status {
		if _, err = io.WriteString(d.out, "status should be "); err != nil {
			return err
		}
		status, err = StatusString(expectedStatus)
		if err != nil {
			return err
		}
		if err = writeDriverLine(d.out, status); err != nil {
			return err
		}
		// The diagnostic writer can change the live status before SASSERT.
		if sourceDebugEnabled && expectedStatus != base.Status {
			return ErrAssertion
		}
		return ErrDriverStatusWrong
	}
	if base.Status == StatusOptimal {
		if _, err = io.WriteString(d.out, "cost = "); err != nil {
			return err
		}
		cost, err := d.floatCost()
		if err != nil {
			return err
		}
		if err = writeDriverLine(d.out, floatLPNumberText(cost)); err != nil {
			return err
		}
		names := make([]string, 0, len(solution))
		for name := range solution {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			expected := solution[name]
			value, err := d.floatValue(name, false)
			if err != nil {
				return err
			}
			if math.Abs(expected-value) >= 0.000001 {
				if _, err = io.WriteString(d.out, "expected:"+name+"="+floatLPNumberText(expected)+", got "); err != nil {
					return err
				}
				value, err = d.floatValue(name, false)
				if err != nil {
					return err
				}
				if err = writeDriverLine(d.out, floatLPNumberText(value)); err != nil {
					return err
				}
			}
			if sourceDebugEnabled {
				value, err = d.floatValue(name, false)
				if err != nil {
					return err
				}
				if !(math.Abs(expected-value) < 0.000001) {
					return ErrAssertion
				}
			}
		}
		if len(d.floatReader.ColumnNames()) < 20 {
			if err = d.printFloatX(false); err != nil {
				return err
			}
		}
	}
	return d.Close()
}
