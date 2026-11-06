package lp

// Source: src/test/lp.cpp:1151-1227. Concrete double/double MPS driver.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (d *MPSFileDriver) floatBase() *FloatLPSolver {
	if d.floatDual != nil {
		return d.floatDual.FloatLPSolver
	}
	return d.floatPrimal.FloatLPSolver
}

func (d *MPSFileDriver) floatCost() (float64, error) {
	if d.floatDual != nil {
		return d.floatDual.GetCurrentCost()
	}
	return d.floatPrimal.GetCurrentCost()
}

func (d *MPSFileDriver) floatValue(name string, comparison bool) (float64, error) {
	base := d.floatBase()
	if comparison {
		base = d.floatComparison.FloatLPSolver
	}
	j, ok := base.NamesToColumns[name]
	if !ok {
		return 0, z3exception.NewDefaultException(fmt.Sprintf("get_column_value_by_name %s", name))
	}
	if comparison {
		return d.floatComparison.GetColumnValue(j)
	}
	if d.floatDual != nil {
		return d.floatDual.GetColumnValue(j)
	}
	return d.floatPrimal.GetColumnValue(j)
}

func (d *MPSFileDriver) printFloatX(comparison bool) error {
	for _, name := range d.floatReader.ColumnNames() {
		if _, err := io.WriteString(d.out, name+"="); err != nil {
			return err
		}
		value, err := d.floatValue(name, comparison)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(d.out, floatLPNumberText(value)+" "); err != nil {
			return err
		}
	}
	return writeDriverLine(d.out, "")
}

func (d *MPSFileDriver) compareFloatValues() error {
	for _, name := range d.floatReader.ColumnNames() {
		a, err := d.floatValue(name, true)
		if err != nil {
			return err
		}
		b, err := d.floatValue(name, false)
		if err != nil {
			return err
		}
		if !DriverValuesOnePercentClose(a, b) {
			if err = writeDriverLine(d.out, "different values for "+name+":"+floatLPNumberText(a)+" and "+floatLPNumberText(b)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *MPSFileDriver) runFloat(compare bool) error {
	d.floatReader = OpenFloatMPSReader(d.Name)
	d.floatReader.Messages = d.out
	if err := d.floatReader.Read(); err != nil {
		return err
	}
	if !d.floatReader.IsOK() {
		return writeDriverLine(d.out, "cannot process "+d.Name)
	}
	var err error
	if d.dual {
		d.floatDual, err = d.floatReader.CreateDualSolver()
	} else {
		d.floatPrimal, err = d.floatReader.CreatePrimalSolver()
	}
	if err != nil {
		return err
	}
	base := d.floatBase()
	base.Settings.MessageOut, base.Settings.DebugOut = d.out, d.out
	if err = base.SetupFromDriver(d.maxIterations, d.timeLimit, d.minimum, d.Arguments, d.out); err != nil {
		return err
	}
	if err = d.watch.Start(); err != nil {
		return err
	}
	if d.dual {
		if err = writeDriverLine(d.out, "solving for dual"); err != nil {
			return err
		}
		err = d.floatDual.FindMaximalSolution()
	} else {
		err = d.floatPrimal.FindMaximalSolution()
	}
	if err != nil {
		return err
	}
	if err = d.watch.Stop(); err != nil {
		return err
	}
	span, err := d.watch.Seconds()
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
		if len(d.floatReader.ColumnNames()) < 20 {
			if err = d.printFloatX(false); err != nil {
				return err
			}
		}
		cost, err := d.floatCost()
		if err != nil {
			return err
		}
		if d.minimum {
			cost = -cost
		}
		if err = writeDriverLine(d.out, "cost = "+floatLPNumberText(cost)); err != nil {
			return err
		}
	}
	if _, err = io.WriteString(d.out, "processed in "); err != nil {
		return err
	}
	if _, err = io.WriteString(d.out, floatLPNumberText(span/1000)+" seconds, running for "); err != nil {
		return err
	}
	if _, err = io.WriteString(d.out, fmt.Sprintf("%d iterations  one iter for ", base.TotalIterations)); err != nil {
		return err
	}
	if err = writeDriverLine(d.out, floatLPNumberText(span/float64(base.TotalIterations))+" ms"); err != nil {
		return err
	}
	if compare {
		return d.compareWithPrimal()
	}
	return nil
}

func (d *MPSFileDriver) compareWithPrimal() error {
	var err error
	d.floatComparison, err = d.floatReader.CreatePrimalSolver()
	if err != nil {
		return err
	}
	p := d.floatComparison
	p.Settings.MessageOut, p.Settings.DebugOut = d.out, d.out
	if err = p.SetupFromDriver(d.maxIterations, d.timeLimit, d.minimum, d.Arguments, d.out); err != nil {
		return err
	}
	if err = p.FindMaximalSolution(); err != nil {
		return err
	}
	base := d.floatBase()
	if base.Status != p.Status {
		if _, err = io.WriteString(d.out, "statuses are different: dual "); err != nil {
			return err
		}
		a, err := StatusString(base.Status)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(d.out, a+" primal = "); err != nil {
			return err
		}
		b, err := StatusString(p.Status)
		if err != nil {
			return err
		}
		return writeDriverLine(d.out, b)
	}
	if base.Status != StatusOptimal {
		return nil
	}
	cost, err := d.floatCost()
	if err != nil {
		return err
	}
	if d.minimum {
		cost = -cost
	}
	primalCost, err := p.GetCurrentCost()
	if err != nil {
		return err
	}
	if d.minimum {
		primalCost = -primalCost
	}
	if err = writeDriverLine(d.out, "primal cost = "+floatLPNumberText(primalCost)); err != nil {
		return err
	}
	if !DriverValuesOnePercentClose(cost, primalCost) {
		if err = d.compareFloatValues(); err != nil {
			return err
		}
		if err = d.printFloatX(true); err != nil {
			return err
		}
		if err = writeDriverLine(d.out, "dual cost is "+floatLPNumberText(cost)+", but primal cost is "+floatLPNumberText(primalCost)); err != nil {
			return err
		}
		return sourceAssertionFailure()
	}
	return nil
}
