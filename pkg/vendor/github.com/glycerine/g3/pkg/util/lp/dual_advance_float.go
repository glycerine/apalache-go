package lp

// Source: src/util/lp/lp_dual_core_solver.hpp:207-235,331-375,411-548,731-740.
// Floating dual pivot/beta, basis update and recovery use the actual shared LU.

import (
	"fmt"
	"math"
)

func (m *FloatDualCoreSolver) DSEFTran() error {
	if m.Factorization == nil {
		return ErrBounds
	}
	return m.Factorization.SolveByForTIndexedOnly(m.PivotRowB1, m.Settings)
}
func (m *FloatDualCoreSolver) CalculateBetaRPrecisely() error {
	t := 0.0
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(m.PivotRowB1.Data)) {
			return ErrBounds
		}
		b := m.PivotRowB1.Data[i]
		t += b * b
	}
	if m.R < 0 || uint64(m.R) >= uint64(len(m.Betas)) {
		return ErrBounds
	}
	m.Betas[m.R] = t
	return nil
}
func (m *FloatDualCoreSolver) InitBetaPrecisely(i uint32) error {
	vec := make([]float64, m.A.RowCount())
	if uint64(i) >= uint64(len(vec)) {
		return ErrBounds
	}
	vec[i] = 1
	if m.Factorization == nil {
		return ErrBounds
	}
	if err := m.Factorization.SolveYBWithErrorCheck(vec, *m.Basis); err != nil {
		return err
	}
	beta := 0.0
	for _, v := range vec {
		beta += v * v
	}
	if uint64(i) >= uint64(len(m.Betas)) {
		return ErrBounds
	}
	m.Betas[i] = beta
	return nil
}
func (m *FloatDualCoreSolver) InitBetasPrecisely() error {
	for i := m.A.RowCount(); i > 0; {
		i--
		if err := m.InitBetaPrecisely(i); err != nil {
			return err
		}
	}
	return nil
}
func (m *FloatDualCoreSolver) UpdateBetas() error {
	q := uint32(m.Q)
	if uint64(q) >= uint64(len(m.PivotRow.Data)) {
		return ErrBounds
	}
	inv := 1 / m.PivotRow.Data[q]
	if m.R < 0 || uint64(m.R) >= uint64(len(m.Betas)) {
		return ErrBounds
	}
	betaR := 0.0001
	// std::max(floor, expression) chooses floor for a NaN expression.
	v := (m.Betas[m.R] * inv) * inv
	if betaR < v {
		betaR = v
	}
	m.Betas[m.R] = betaR
	k := -2 * inv
	for i := m.A.RowCount(); i > 0; {
		i--
		if int32(i) == m.R {
			continue
		}
		if uint64(i) >= uint64(len(m.Ed.Data)) {
			return ErrBounds
		}
		a := m.Ed.Data[i]
		if uint64(i) >= uint64(len(m.Betas)) || uint64(i) >= uint64(len(m.PivotRowB1.Data)) {
			return ErrBounds
		}
		m.Betas[i] += a * (a*betaR + k*m.PivotRowB1.Data[i])
		if m.Betas[i] < 0.0001 {
			m.Betas[i] = 0.0001
		}
	}
	return nil
}
func (m *FloatDualCoreSolver) DIsCorrect() (bool, error) {
	if err := m.SolveYB(m.Y); err != nil {
		return false, err
	}
	for _, j := range *m.NBasis {
		if uint64(j) >= uint64(len(*m.Costs)) {
			return false, ErrBounds
		}
		dot, err := m.A.DotColumn(m.Y, j)
		if err != nil {
			return false, err
		}
		d := (*m.Costs)[j] - dot
		if uint64(j) >= uint64(len(m.D)) {
			return false, ErrBounds
		}
		if math.Abs(d-m.D[j]) >= 0.001 {
			if m.Settings.DebugOut != nil {
				if _, err := fmt.Fprintf(m.Settings.DebugOut, "total_iterations = %d\nd[%d] = %s but should be %s\n", m.TotalIterations, j, floatLPNumberText(m.D[j]), floatLPNumberText(d)); err != nil {
					return false, err
				}
			}
			return false, nil
		}
	}
	return true, nil
}
func (m *FloatDualCoreSolver) SnapRunawayNonBasicColumn(j uint32) (bool, error) {
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed, ColumnLowBound:
		at, err := m.dualAtBound(j, false)
		if err != nil {
			return false, err
		}
		if !at {
			(*m.X)[j] = (*m.LowBounds)[j]
			return true, nil
		}
	case ColumnBoxed:
		x, low, err := m.dualBound(j, false)
		if err != nil {
			return false, err
		}
		_, upper, err := m.dualBound(j, true)
		if err != nil {
			return false, err
		}
		closer := math.Abs(low-x) < math.Abs(upper-x)
		at, err := m.dualAtBound(j, !closer)
		if err != nil {
			return false, err
		}
		if !at {
			(*m.X)[j] = low
			return true, nil
		} // Source also writes low when nearer upper.
	case ColumnUpperBound:
		at, err := m.dualAtBound(j, true)
		if err != nil {
			return false, err
		}
		if !at {
			(*m.X)[j] = (*m.UpperBounds)[j]
			return true, nil
		}
	}
	return false, nil
}
func (m *FloatDualCoreSolver) ProblemIsDualFeasible() (bool, error) {
	for _, j := range *m.NBasis {
		ok, err := m.ColumnIsDualFeasible(j)
		if err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}
func (m *FloatDualCoreSolver) RecoverLeaving() error {
	q := uint32(m.Q)
	switch m.EnteringBoundaryPosition {
	case PositionAtLowBound, PositionAtFixed, PositionAtUpperBound:
		_, b, err := m.dualBound(q, m.EnteringBoundaryPosition == PositionAtUpperBound)
		if err != nil {
			return err
		}
		(*m.X)[q] = b
		return nil
	case PositionFreeOfBounds:
		if uint64(q) >= uint64(len(*m.X)) {
			return ErrBounds
		}
		(*m.X)[q] = 0
		fallthrough
	default:
		return sourceAssertionFailure()
	}
}
func (m *FloatDualCoreSolver) RevertToPreviousBasis() error {
	if m.Settings.DebugOut != nil {
		if _, err := fmt.Fprintf(m.Settings.DebugOut, "revert to previous basis on ( %d, %d)\n", m.P, m.Q); err != nil {
			return err
		}
	}
	if err := m.ChangeBasisUnconditionally(uint32(m.P), uint32(m.Q)); err != nil {
		return err
	}
	if err := m.InitLU(); err != nil {
		return err
	}
	if m.Factorization.GetStatus() != LUOK {
		m.Status = StatusFloatingPointError
		return nil
	}
	if err := m.RecoverLeaving(); err != nil {
		return err
	}
	ok, err := m.FindXBySolving()
	if err != nil {
		return err
	}
	if !ok {
		m.Status = StatusFloatingPointError
		return nil
	}
	if err = m.RecalculateXBAndD(); err != nil {
		return err
	}
	return m.InitBetasPrecisely()
}
func (m *FloatDualCoreSolver) BasisChangeAndUpdate() (bool, error) {
	if err := m.UpdateBetas(); err != nil {
		return false, err
	}
	if err := m.UpdateDAndXB(); err != nil {
		return false, err
	}
	q := uint32(m.Q)
	if uint64(q) >= uint64(len(m.PivotRow.Data)) {
		return false, ErrBounds
	}
	m.ThetaP = m.Delta / m.PivotRow.Data[q]
	if err := m.ApplyFlips(); err != nil {
		return false, err
	}
	ok, err := m.UpdateBasisAndX(m.Q, m.P, m.ThetaP)
	if err != nil {
		return false, err
	}
	if !ok {
		if err = m.InitBetasPrecisely(); err != nil {
			return false, err
		}
		return false, nil
	}
	snapped, err := m.SnapRunawayNonBasicColumn(uint32(m.P))
	if err != nil {
		return false, err
	}
	if snapped {
		ok, err = m.FindXBySolving()
		if err != nil {
			return false, err
		}
		if !ok {
			if err = m.RevertToPreviousBasis(); err != nil {
				return false, err
			}
			m.IterationsNoCostGrowth++
			return false, nil
		}
	}
	ok, err = m.ProblemIsDualFeasible()
	if err != nil {
		return false, err
	}
	if !ok {
		if err = m.RevertToPreviousBasis(); err != nil {
			return false, err
		}
		m.IterationsNoCostGrowth++
		return false, nil
	}
	if sourceDebugEnabled {
		ok, err = m.DIsCorrect()
		if err != nil {
			return false, err
		}
		if !ok {
			return false, ErrAssertion
		}
	}
	return true, nil
}
func (m *FloatDualCoreSolver) AdvanceOnKnownP() (bool, error) {
	if m.Done() {
		return true, nil
	}
	if err := m.CalculatePivotRowOfBInverse(uint32(m.R)); err != nil {
		return false, err
	}
	if err := m.CalculatePivotRowWhenInverseReady(uint32(m.R)); err != nil {
		return false, err
	}
	ok, err := m.RatioTest()
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	if err = m.CalculateBetaRPrecisely(); err != nil {
		return false, err
	}
	if err = m.SolveBd(uint32(m.Q)); err != nil {
		return false, err
	}
	comparison, err := m.PivotsInColumnAndRowAreDifferent(m.Q, m.P)
	if err != nil {
		return false, err
	}
	if comparison == 2 {
		if err = sourceAssertionFailure(); err != nil {
			return false, err
		}
	} else if comparison != 0 {
		if sourceDebugEnabled && comparison != 1 {
			return false, ErrAssertion
		}
		if err = m.InitLU(); err != nil {
			return false, err
		}
	}
	if err = m.DSEFTran(); err != nil {
		return false, err
	}
	return m.BasisChangeAndUpdate()
}
