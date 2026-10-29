package lp

// Source: src/util/lp/lp_core_solver_base.h:37,153-155;
// lp_core_solver_base.hpp:350-377,449-458,516-524; lu.h:49-55,256-258;
// lp_settings.h:344-348. Concrete mpq/mpq specialization.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (m *RationalCoreSolverBase) IncTotalIterations() (uint32, error) {
	if err := m.ready(); err != nil {
		return 0, err
	}
	m.Settings.Stats.TotalIterations++
	old := m.TotalIterations
	m.TotalIterations++
	return old, nil
}

func (m *RationalCoreSolverBase) TimeIsOver() (over bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	over, err = m.Settings.CancelFlag()
	if err != nil {
		return false, err
	}
	if over {
		m.Status = StatusTimeExhausted
	}
	return over, nil
}

func (m *RationalCoreSolverBase) copyStatisticsArgument(cost *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.statisticsArgument = NewIndexedRationalVector(m.manager, 1)
	return m.manager.Copy(&m.statisticsArgument.Data[0], cost)
}

func (m *RationalCoreSolverBase) statisticsCostNumber() (number float64, err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return 0, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return m.manager.Float64(&m.statisticsArgument.Data[0])
}

// PrintStatistics snapshots the source by-value argument before output. Writer
// chunks retain the evaluation points of numeral conversion and LU inspection.
func (m *RationalCoreSolverBase) PrintStatistics(prefix *string, cost *numeral.Rational, out io.Writer) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.statisticsArgument != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	if cost == nil {
		return ErrBounds
	}
	if err = m.copyStatisticsArgument(cost); err != nil {
		return err
	}
	if out == nil {
		return ErrBounds
	}
	if prefix != nil {
		if _, err = fmt.Fprintf(out, "%s ", *prefix); err != nil {
			return err
		}
	}
	if _, err = fmt.Fprintf(out, "iterations = %d, cost = ", m.TotalIterations-1); err != nil {
		return err
	}
	number, e := m.statisticsCostNumber()
	if e != nil {
		return e
	}
	if _, err = fmt.Fprintf(out, "%s, nonzeros = ", floatLPNumberText(number)); err != nil {
		return err
	}
	n := uint32(0)
	if m.Factorization != nil {
		if err = m.Factorization.ready(); err != nil {
			return err
		}
		for i := len(m.Factorization.U.Rows); i > 0; {
			i--
			n += uint32(len(m.Factorization.U.Rows[i]))
		}
	} else {
		for _, row := range m.A.Rows {
			n += uint32(len(row))
		}
	}
	if _, err = fmt.Fprintf(out, "%d\n", n); err != nil {
		return err
	}
	if err = m.statisticsArgument.Close(); err != nil {
		return err
	}
	m.statisticsArgument = nil
	return nil
}

func (m *RationalCoreSolverBase) PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(prefix *string, out io.Writer) (over bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	if m.statisticsCost != nil {
		return false, ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	old, e := m.IncTotalIterations()
	if e != nil {
		return false, e
	}
	if m.Settings.ReportFrequency != 0 && m.Settings.PrintStatistics && old%uint32(m.Settings.ReportFrequency) == 0 {
		m.statisticsCost = NewIndexedRationalVector(m.manager, 1)
		if err = m.GetCost(&m.statisticsCost.Data[0]); err != nil {
			return false, err
		}
		if err = m.PrintStatistics(prefix, &m.statisticsCost.Data[0], out); err != nil {
			return false, err
		}
		if err = m.statisticsCost.Close(); err != nil {
			return false, err
		}
		m.statisticsCost = nil
	}
	return m.TimeIsOver()
}
