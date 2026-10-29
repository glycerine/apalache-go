package lp

// Source: src/test/lp.cpp:1279-1303,1711-1720.

import (
	"io"
)

// RunRationalDefault runs the auxiliary solve_mps_in_rational consumer. It
// ignores Arguments and retains the solver's constructor defaults: no setup,
// minimum-cost flip, limits or stopwatch. Failed owners remain until Close.
func (d *MPSFileDriver) RunRationalDefault(dual bool) error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started, d.dual = true, dual
	if err := writeDriverLine(d.out, "solving "+d.Name); err != nil {
		return err
	}
	d.rationalReader = OpenRationalMPSReader(d.manager, d.Name)
	d.rationalReader.Messages = d.out
	if err := d.rationalReader.Read(); err != nil {
		return err
	}
	if !d.rationalReader.IsOK() {
		if err := writeDriverLine(d.out, "cannot process "+d.Name); err != nil {
			return err
		}
		return d.Close()
	}
	var err error
	if dual {
		d.rationalDual, err = d.rationalReader.CreateDualSolver()
	} else {
		d.rationalPrimal, err = d.rationalReader.CreatePrimalSolver()
	}
	if err != nil {
		return err
	}
	base := d.rationalBase()
	base.Settings.MessageOut, base.Settings.DebugOut = d.out, d.out
	if dual {
		err = d.rationalDual.FindMaximalSolution()
	} else {
		err = d.rationalPrimal.FindMaximalSolution()
	}
	if err != nil {
		return err
	}
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
	if base.Status == StatusOptimal {
		names := d.rationalReader.ColumnNames()
		if len(names) < 20 {
			for _, name := range names {
				if _, err = io.WriteString(d.out, name+"="); err != nil {
					return err
				}
				if err = d.reportDefaultRationalNumber(&name); err != nil {
					return err
				}
			}
		}
		if err = writeDriverLine(d.out, ""); err != nil {
			return err
		}
		if _, err = io.WriteString(d.out, "cost = "); err != nil {
			return err
		}
		if err = d.reportDefaultRationalNumber(nil); err != nil {
			return err
		}
	}
	return d.Close()
}

// Each returned by-value rational survives through its entire insertion chain
// (including the trailing space or endl) and dies before the next expression.
func (d *MPSFileDriver) reportDefaultRationalNumber(name *string) error {
	d.work = NewIndexedRationalVector(d.manager, 1)
	value := &d.work.Data[0]
	var err error
	if name == nil {
		if d.dual {
			err = d.rationalDual.GetCurrentCost(value)
		} else {
			err = d.rationalPrimal.GetCurrentCost(value)
		}
	} else if d.dual {
		err = d.rationalDual.GetColumnValueByName(*name, value)
	} else {
		err = d.rationalPrimal.GetColumnValueByName(*name, value)
	}
	if err != nil {
		return err
	}
	before, err := d.calls.begin(d.manager)
	if err != nil {
		return err
	}
	number, e := d.manager.Float64(value)
	if err = d.calls.after(d.manager, before, e); err != nil {
		return err
	}
	if name == nil {
		err = writeDriverLine(d.out, floatLPNumberText(number))
	} else {
		_, err = io.WriteString(d.out, floatLPNumberText(number)+" ")
	}
	if err != nil {
		return err
	}
	if err = d.work.Close(); err != nil {
		return err
	}
	d.work = nil
	return nil
}
