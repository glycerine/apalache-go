package lp

// Source: src/util/lp/lp_core_solver_base.h:37,153-155,689-697;
// lp_core_solver_base.hpp:350-377,449-458; lu.h:49-55,256-258.

import (
	"fmt"
	"io"
	"math"
	"strconv"
)

func (m *FloatCoreSolverBase) IncTotalIterations() uint32 {
	m.Settings.Stats.TotalIterations++
	old := m.TotalIterations
	m.TotalIterations++
	return old
}

func (m *FloatCoreSolverBase) GetCost() (float64, error) {
	if sourceDebugEnabled && len(*m.Costs) != len(*m.X) {
		return 0, ErrAssertion
	}
	cost := 0.0
	for i, v := range *m.Costs {
		if i >= len(*m.X) {
			return cost, ErrBounds
		}
		cost += v * (*m.X)[i]
	}
	return cost, nil
}

func (m *FloatCoreSolverBase) TimeIsOver() (bool, error) {
	over, err := m.Settings.CancelFlag()
	if err != nil {
		return false, err
	}
	if over {
		m.Status = StatusTimeExhausted
	}
	return over, nil
}

func (m *FloatCoreSolverBase) CostsOnNBasisAreZeros() error {
	if !sourceDebugEnabled {
		return nil
	}
	ok, err := m.BasisHeadingIsCorrect()
	if err != nil {
		return err
	}
	if !ok {
		return ErrAssertion
	}
	for j := uint32(0); j < m.A.ColumnCount(); j++ {
		if uint64(j) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		if (*m.Heading)[j] < 0 {
			if uint64(j) >= uint64(len(*m.Costs)) {
				return ErrBounds
			}
			if (*m.Costs)[j] != 0 {
				return ErrAssertion
			}
		}
	}
	return nil
}

// PrintStatistics uses ordinary Go writers and classic six-digit number text.
// Stream locale/format flags are outside this Go boundary. A nil writer is
// allowed only when statistics are disabled by the caller's source gate.
func (m *FloatCoreSolverBase) PrintStatistics(prefix *string, cost float64, out io.Writer) error {
	if out == nil {
		return ErrBounds
	}
	if prefix != nil {
		if _, err := fmt.Fprintf(out, "%s ", *prefix); err != nil {
			return err
		}
	}
	formatted := floatLPNumberText(cost)
	if _, err := fmt.Fprintf(out, "iterations = %d, cost = %s, nonzeros = ", m.TotalIterations-1, formatted); err != nil {
		return err
	}
	n := uint32(0)
	if m.Factorization != nil {
		if err := m.Factorization.ready(); err != nil {
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
	_, err := fmt.Fprintf(out, "%d\n", n)
	return err
}

func floatLPNumberText(cost float64) string {
	formatted := strconv.FormatFloat(cost, 'g', 6, 64)
	if math.IsInf(cost, 1) {
		formatted = "inf"
	}
	if math.IsInf(cost, -1) {
		formatted = "-inf"
	}
	if math.IsNaN(cost) {
		formatted = "nan"
		if math.Signbit(cost) {
			formatted = "-nan"
		}
	}
	return formatted
}

func (m *FloatCoreSolverBase) PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(prefix *string, out io.Writer) (bool, error) {
	old := m.IncTotalIterations()
	if m.Settings.ReportFrequency != 0 && m.Settings.PrintStatistics && old%uint32(m.Settings.ReportFrequency) == 0 {
		cost, err := m.GetCost()
		if err != nil {
			return false, err
		}
		if err := m.PrintStatistics(prefix, cost, out); err != nil {
			return false, err
		}
	}
	return m.TimeIsOver()
}
