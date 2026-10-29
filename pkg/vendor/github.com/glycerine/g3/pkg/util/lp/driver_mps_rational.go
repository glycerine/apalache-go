package lp

// Source: src/test/lp.cpp:1229-1254. Concrete retained mpq/mpq MPS driver.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (d *MPSFileDriver) rationalBase() *RationalLPSolver {
	if d.rationalDual != nil {
		return d.rationalDual.RationalLPSolver
	}
	return d.rationalPrimal.RationalLPSolver
}

func (d *MPSFileDriver) runRational() error {
	d.rationalReader = OpenRationalMPSReader(d.manager, d.Name)
	d.rationalReader.Messages = d.out
	if err := d.rationalReader.Read(); err != nil {
		return err
	}
	if !d.rationalReader.IsOK() {
		return writeDriverLine(d.out, "cannot process "+d.Name)
	}
	var err error
	if d.dual {
		d.rationalDual, err = d.rationalReader.CreateDualSolver()
	} else {
		d.rationalPrimal, err = d.rationalReader.CreatePrimalSolver()
	}
	if err != nil {
		return err
	}
	base := d.rationalBase()
	base.Settings.MessageOut, base.Settings.DebugOut = d.out, d.out
	if err = base.SetupFromDriver(d.maxIterations, d.timeLimit, d.minimum, d.Arguments, d.out); err != nil {
		return err
	}
	if err = d.watch.Start(); err != nil {
		return err
	}
	if d.dual {
		err = d.rationalDual.FindMaximalSolution()
	} else {
		err = d.rationalPrimal.FindMaximalSolution()
	}
	if err != nil {
		return err
	}
	if _, err = io.WriteString(d.out, "Status: "); err != nil {
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
		if err = d.reportRationalCost(); err != nil {
			return err
		}
	}
	// Source prints this prefix before querying the running watch. Seconds
	// stops and restarts it; the retained /1000 is not a unit correction.
	if _, err = io.WriteString(d.out, "processed in "); err != nil {
		return err
	}
	span, err := d.watch.Seconds()
	if err != nil {
		return err
	}
	if _, err = io.WriteString(d.out, floatLPNumberText(span/1000)+" seconds, running for "); err != nil {
		return err
	}
	return writeDriverLine(d.out, fmt.Sprintf("%d iterations", base.TotalIterations))
}

func (d *MPSFileDriver) reportRationalCost() (err error) {
	d.work = NewIndexedRationalVector(d.manager, 2)
	cost, negative := &d.work.Data[0], &d.work.Data[1]
	// Owning solver getters retain their own captures; no parent encloses them.
	if d.dual {
		err = d.rationalDual.GetCurrentCost(cost)
	} else {
		err = d.rationalPrimal.GetCurrentCost(cost)
	}
	if err != nil {
		return err
	}
	if d.minimum {
		before, e := d.calls.begin(d.manager)
		if e != nil {
			return e
		}
		err = d.manager.Copy(negative, cost)
		if err = d.calls.after(d.manager, before, err); err != nil {
			return err
		}
		before, e = d.calls.begin(d.manager)
		if e != nil {
			return e
		}
		err = d.manager.Neg(negative)
		if err = d.calls.after(d.manager, before, err); err != nil {
			return err
		}
		before, e = d.calls.begin(d.manager)
		if e != nil {
			return e
		}
		err = d.manager.Copy(cost, negative)
		if err = d.calls.after(d.manager, before, err); err != nil {
			return err
		}
		if err = d.manager.Del(negative); err != nil {
			return err
		}
		*negative = numeral.NewRational()
	}
	if _, err = io.WriteString(d.out, "cost = "); err != nil {
		return err
	}
	before, e := d.calls.begin(d.manager)
	if e != nil {
		return e
	}
	number, e := d.manager.Float64(cost)
	if err = d.calls.after(d.manager, before, e); err != nil {
		return err
	}
	if err = writeDriverLine(d.out, floatLPNumberText(number)); err != nil {
		return err
	}
	// The lexical objective dies at the source if-block end, before timing.
	if err = d.work.Close(); err != nil {
		return err
	}
	d.work = nil
	return nil
}
