package lp

// Source: src/util/lp/lp_core_solver_base.h:37,153-155;
// lp_core_solver_base.hpp:350-377,449-458,516-524; lu.h:49-55,256-258;
// lp_settings.h:336-341. Concrete mpq/impq specialization.

import (
	"fmt"
	"io"
)

// GetCost evaluates the dense scalar-left dot product, then assigns its
// completed result. All inputs remain live through their source iteration.
func (m *PairCoreSolverBase) GetCost(out *RationalPair) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	if sourceDebugEnabled && len(*m.Costs) != len(*m.X) {
		return ErrAssertion
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.valueWork = NewIndexedPairVector(m.manager, 2)
	sum, product := &m.valueWork.Data[0], &m.valueWork.Data[1]
	for i := range *m.Costs {
		if i >= len(*m.X) {
			return ErrBounds
		}
		m.valueScalars = NewIndexedRationalVector(m.manager, 3)
		tmp, x, y := &m.valueScalars.Data[0], &m.valueScalars.Data[1], &m.valueScalars.Data[2]
		if err = etaRationalProduct(m.manager, tmp, x, &(*m.Costs)[i], &(*m.X)[i].X); err != nil {
			return err
		}
		if err = etaRationalProduct(m.manager, tmp, y, &(*m.Costs)[i], &(*m.X)[i].Y); err != nil {
			return err
		}
		if err = m.manager.Copy(&product.X, x); err != nil {
			return err
		}
		if err = m.manager.Copy(&product.Y, y); err != nil {
			return err
		}
		if err = m.valueScalars.Close(); err != nil {
			return err
		}
		m.valueScalars = nil
		if err = sum.Add(m.manager, sum, product); err != nil {
			return err
		}
		if err = product.Del(m.manager); err != nil {
			return err
		}
	}
	if err = out.Copy(m.manager, sum); err != nil {
		return err
	}
	if err = m.valueWork.Close(); err != nil {
		return err
	}
	m.valueWork = nil
	return nil
}

func (m *PairCoreSolverBase) IncTotalIterations() (uint32, error) {
	if err := m.ready(); err != nil {
		return 0, err
	}
	m.Settings.Stats.TotalIterations++
	old := m.TotalIterations
	m.TotalIterations++
	return old, nil
}

func (m *PairCoreSolverBase) TimeIsOver() (over bool, err error) {
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

func (m *PairCoreSolverBase) copyStatisticsArgument(cost *RationalPair) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.statisticsArgument = NewIndexedPairVector(m.manager, 1)
	return m.statisticsArgument.Data[0].Copy(m.manager, cost)
}

func (m *PairCoreSolverBase) statisticsCostNumber() (number float64, err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return 0, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.pivotWork = NewIndexedRationalVector(m.manager, 4)
	thousand, tmp, quotient, sum := &m.pivotWork.Data[0], &m.pivotWork.Data[1], &m.pivotWork.Data[2], &m.pivotWork.Data[3]
	cost := &m.statisticsArgument.Data[0]
	if err = m.manager.SetInt32(thousand, 1000); err != nil {
		return 0, err
	}
	if err = etaRationalQuotient(m.manager, tmp, quotient, &cost.Y, thousand); err != nil {
		return 0, err
	}
	if err = etaRationalSum(m.manager, tmp, sum, &cost.X, quotient); err != nil {
		return 0, err
	}
	number, err = m.manager.Float64(sum)
	if err != nil {
		return 0, err
	}
	if err = m.pivotWork.Close(); err != nil {
		return 0, err
	}
	m.pivotWork = nil
	return number, nil
}

// PrintStatistics snapshots the source by-value argument before output. Writer
// chunks retain the evaluation points of numeral conversion and LU inspection.
func (m *PairCoreSolverBase) PrintStatistics(prefix *string, cost *RationalPair, out io.Writer) (err error) {
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

func (m *PairCoreSolverBase) PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(prefix *string, out io.Writer) (over bool, err error) {
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
		m.statisticsCost = NewIndexedPairVector(m.manager, 1)
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

func (m *PairCoreSolverBase) CalcCurrentXIsFeasibleIncludeNonBasis() (feasible bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	for j := m.A.ColumnCount(); j > 0; {
		j--
		ok, e := m.ColumnIsFeasible(j)
		if e != nil {
			return false, e
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}
