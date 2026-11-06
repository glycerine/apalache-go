package lp

// Source: src/util/lp/lp_dual_core_solver.hpp:168-204,550-556,751-779.

import "math"

func (m *FloatDualCoreSolver) GetNumberOfRowsToTryForLeaving() (uint32, error) {
	s := m.A.RowCount()
	if s > 300 {
		v := (float64(s) / 100.0) * float64(m.Settings.PercentOfEnteringToCheck)
		if v < 0 || v >= float64(math.MaxUint32)+1 || math.IsNaN(v) {
			return 0, ErrBounds
		}
		s = uint32(v)
	}
	random := m.Settings.RandomNext()
	if s == 0 {
		return 0, ErrBounds
	} // Source modulo-zero is undefined, after RNG.
	return random%s + 1, nil
}

// The budget counts record-breaking scores, not visited rows. The do/while
// source examines one row even with a zero budget; forbidden rows still wrap.
func (m *FloatDualCoreSolver) PricingLoop(numberOfRowsToTry, offset uint32) error {
	m.R = -1
	rows := m.A.RowCount()
	if rows == 0 || offset >= rows {
		return ErrBounds
	}
	max := 0.0
	left := numberOfRowsToTry
	i := offset
	for {
		if !m.ForbiddenRows[i] {
			score, err := m.PricingForRow(i)
			if err != nil {
				return err
			}
			if score > max {
				max = score
				m.R = int32(i)
				if left > 0 {
					left--
				}
			}
		}
		i++
		if i == rows {
			i = 0
		}
		if i == offset || left == 0 {
			break
		}
	}
	if m.R == -1 {
		if m.Status != StatusUnstable {
			m.Status = StatusOptimal
		}
		return nil
	}
	if uint64(m.R) >= uint64(len(*m.Basis)) {
		return ErrBounds
	}
	m.P = int32((*m.Basis)[m.R])
	delta, err := m.GetDelta()
	if err != nil {
		return err
	}
	m.Delta = delta
	ok, err := m.AdvanceOnKnownP()
	if err != nil {
		return err
	}
	if ok {
		m.ForbiddenRows = nil
		return nil
	}
	if m.Status == StatusFloatingPointError {
		return nil
	}
	m.Status = StatusUnstable
	if m.ForbiddenRows == nil {
		m.ForbiddenRows = make(map[uint32]bool)
	}
	m.ForbiddenRows[uint32(m.R)] = true
	return nil
}
func (m *FloatDualCoreSolver) OneIteration() error {
	n, err := m.GetNumberOfRowsToTryForLeaving()
	if err != nil {
		return err
	}
	random := m.Settings.RandomNext()
	rows := m.A.RowCount()
	if rows == 0 {
		return ErrBounds
	}
	offset := random % rows
	if m.Status == StatusTentativeDualUnbounded {
		n = rows
	} else {
		m.Status = StatusFeasible
	}
	if err = m.PricingLoop(n, offset); err != nil {
		return err
	}
	if sourceDebugEnabled {
		ok, err := m.ProblemIsDualFeasible()
		if err != nil {
			return err
		}
		if !ok {
			return ErrAssertion
		}
	}
	return nil
}

// Solve runs the source floating dual core. It expects prepared primal values,
// reduced costs and betas; the outer two-stage dual driver is separate.
func (m *FloatDualCoreSolver) Solve() (uint32, error) {
	if sourceDebugEnabled {
		ok, err := m.DIsCorrect()
		if err != nil {
			return m.TotalIterations, err
		}
		if !ok {
			return m.TotalIterations, ErrAssertion
		}
		ok, err = m.ProblemIsDualFeasible()
		if err != nil {
			return m.TotalIterations, err
		}
		if !ok {
			return m.TotalIterations, ErrAssertion
		}
		ok, err = m.BasisHeadingIsCorrect()
		if err != nil {
			return m.TotalIterations, err
		}
		if !ok {
			return m.TotalIterations, ErrAssertion
		}
	}
	m.TotalIterations = 0
	m.IterationsNoCostGrowth = 0
	for {
		prefix := ""
		over, err := m.PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(&prefix, m.Settings.MessageOut)
		if err != nil || over {
			return m.TotalIterations, err
		}
		if err = m.OneIteration(); err != nil {
			return m.TotalIterations, err
		}
		if m.Status == StatusFloatingPointError || m.Status == StatusDualUnbounded || m.Status == StatusOptimal || m.IterationsNoCostGrowth > m.Settings.MaxIterationsWithoutImprovements || m.TotalIterations > m.Settings.MaxTotalIterations {
			break
		}
	}
	return m.TotalIterations, nil
}
