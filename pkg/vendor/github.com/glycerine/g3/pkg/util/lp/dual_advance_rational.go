package lp

// Source: src/util/lp/lp_dual_core_solver.hpp:213-235,339-350,433-533.
// Concrete retained mpq/mpq basis advancement, snapping and recovery.

import (
	"fmt"
	"io"
)

func dualFlush(out io.Writer) error {
	if f, ok := out.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}
func (m *RationalDualCoreSolver) printDualCostMismatch(j uint32) (err error) {
	if m.Settings == nil {
		return ErrBounds
	}
	out := m.Settings.DebugOut
	if out == nil {
		return nil
	}
	if _, err = fmt.Fprintf(out, "total_iterations = %d\n", m.TotalIterations); err != nil {
		return err
	}
	if err = dualFlush(out); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "d[%d] = ", j); err != nil {
		return err
	}
	text, e := m.manager.String(&m.D.Data[j])
	if e != nil {
		return e
	}
	if _, err = fmt.Fprint(out, text, " but should be "); err != nil {
		return err
	}
	text, e = m.manager.String(&m.diagnosticValues.Data[1])
	if e != nil {
		return e
	}
	if _, err = fmt.Fprintln(out, text); err != nil {
		return err
	}
	return dualFlush(out)
}
func (m *RationalDualCoreSolver) dualCostDeviation(j uint32) (number float64, err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return 0, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 4)
	tmp, difference, absolute, returned := &m.priceWork.Data[0], &m.priceWork.Data[1], &m.priceWork.Data[2], &m.priceWork.Data[3]
	if err = etaRationalDifference(m.manager, tmp, &m.diagnosticValues.Data[1], &(*m.Costs)[j], &m.diagnosticValues.Data[0]); err != nil {
		return 0, err
	}
	if uint64(j) >= uint64(len(m.D.Data)) {
		return 0, ErrBounds
	}
	if err = etaRationalDifference(m.manager, tmp, difference, &m.diagnosticValues.Data[1], &m.D.Data[j]); err != nil {
		return 0, err
	}
	if err = m.manager.Copy(absolute, difference); err != nil {
		return 0, err
	}
	if err = m.manager.Abs(absolute); err != nil {
		return 0, err
	}
	if err = m.manager.Copy(returned, absolute); err != nil {
		return 0, err
	}
	number, err = m.manager.Float64(returned)
	if err != nil {
		return 0, err
	}
	if err = m.priceWork.Close(); err != nil {
		return 0, err
	}
	m.priceWork = nil
	return number, nil
}
func (m *RationalDualCoreSolver) DIsCorrect() (correct bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	if m.diagnosticValues != nil {
		return false, ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	if err = m.SolveYB(m.Y); err != nil {
		return false, err
	}
	for _, j := range *m.NBasis {
		if uint64(j) >= uint64(len(*m.Costs)) {
			return false, ErrBounds
		}
		m.diagnosticValues = NewIndexedRationalVector(m.manager, 2)
		if err = m.A.DotColumn(m.Y.Data, j, &m.diagnosticValues.Data[0]); err != nil {
			return false, err
		}
		number, e := m.dualCostDeviation(j)
		if e != nil {
			return false, e
		}
		if number >= 0.001 {
			if err = m.printDualCostMismatch(j); err != nil {
				return false, err
			}
			if err = m.diagnosticValues.Close(); err != nil {
				return false, err
			}
			m.diagnosticValues = nil
			return false, nil
		}
		if err = m.diagnosticValues.Close(); err != nil {
			return false, err
		}
		m.diagnosticValues = nil
	}
	return true, nil
}
func (m *RationalDualCoreSolver) closerToLowBound(j uint32) (closer bool, err error) {
	if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.LowBounds)) || uint64(j) >= uint64(len(*m.UpperBounds)) {
		return false, ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 7)
	tmp, low, absLow, retLow, up, absUp, retUp := &m.priceWork.Data[0], &m.priceWork.Data[1], &m.priceWork.Data[2], &m.priceWork.Data[3], &m.priceWork.Data[4], &m.priceWork.Data[5], &m.priceWork.Data[6]
	if err = etaRationalDifference(m.manager, tmp, low, &(*m.LowBounds)[j], &(*m.X)[j]); err != nil {
		return false, err
	}
	if err = m.manager.Copy(absLow, low); err != nil {
		return false, err
	}
	if err = m.manager.Abs(absLow); err != nil {
		return false, err
	}
	if err = m.manager.Copy(retLow, absLow); err != nil {
		return false, err
	}
	if err = etaRationalDifference(m.manager, tmp, up, &(*m.UpperBounds)[j], &(*m.X)[j]); err != nil {
		return false, err
	}
	if err = m.manager.Copy(absUp, up); err != nil {
		return false, err
	}
	if err = m.manager.Abs(absUp); err != nil {
		return false, err
	}
	if err = m.manager.Copy(retUp, absUp); err != nil {
		return false, err
	}
	cmp, e := m.manager.Compare(retLow, retUp)
	if e != nil {
		return false, e
	}
	if err = m.priceWork.Close(); err != nil {
		return false, err
	}
	m.priceWork = nil
	return cmp < 0, nil
}
func (m *RationalDualCoreSolver) SnapRunawayNonBasicColumn(j uint32) (snapped bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	upper := false
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed, ColumnLowBound:
	case ColumnBoxed:
		closer, e := m.closerToLowBound(j)
		if e != nil {
			return false, e
		}
		upper = !closer
	case ColumnUpperBound:
		upper = true
	default:
		return false, nil
	}
	at, e := m.dualAtBound(j, upper)
	if e != nil {
		return false, e
	}
	if at {
		return false, nil
	}
	bounds := m.LowBounds
	// Source boxed columns also assign LOW when nearer/equidistant to upper.
	if (*m.ColumnTypes)[j] == ColumnUpperBound {
		bounds = m.UpperBounds
	}
	if uint64(j) >= uint64(len(*bounds)) {
		return false, ErrBounds
	}
	if err = m.copyPricingValue(&(*m.X)[j], &(*bounds)[j]); err != nil {
		return false, err
	}
	return true, nil
}
func (m *RationalDualCoreSolver) RecoverLeaving() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	q := uint32(m.Q)
	switch m.EnteringBoundaryPosition {
	case PositionAtLowBound, PositionAtFixed, PositionAtUpperBound:
		bounds := m.LowBounds
		if m.EnteringBoundaryPosition == PositionAtUpperBound {
			bounds = m.UpperBounds
		}
		if uint64(q) >= uint64(len(*m.X)) || uint64(q) >= uint64(len(*bounds)) {
			return ErrBounds
		}
		return m.copyPricingValue(&(*m.X)[q], &(*bounds)[q])
	case PositionFreeOfBounds:
		if uint64(q) >= uint64(len(*m.X)) {
			return ErrBounds
		}
		if err = m.zeroPricingValue(&(*m.X)[q]); err != nil {
			return err
		}
		fallthrough
	default:
		return sourceAssertionFailure()
	}
}
func (m *RationalDualCoreSolver) RevertToPreviousBasis() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if m.Settings == nil {
		return ErrBounds
	}
	if out := m.Settings.DebugOut; out != nil {
		if _, err = fmt.Fprintf(out, "revert to previous basis on ( %d, %d)\n", m.P, m.Q); err != nil {
			return err
		}
		if err = dualFlush(out); err != nil {
			return err
		}
	}
	if err = m.ChangeBasisUnconditionally(uint32(m.P), uint32(m.Q)); err != nil {
		return err
	}
	if err = m.InitLU(); err != nil {
		return err
	}
	if m.Factorization.GetStatus() != LUOK {
		m.Status = StatusFloatingPointError
		return nil
	}
	if err = m.RecoverLeaving(); err != nil {
		return err
	}
	ok, e := m.FindXBySolving()
	if e != nil {
		return e
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
func (m *RationalDualCoreSolver) assignDualPrimalTheta() (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	q := uint32(m.Q)
	if uint64(q) >= uint64(len(m.PivotRow.Data)) {
		return ErrBounds
	}
	m.priceWork = NewIndexedRationalVector(m.manager, 1)
	if err = etaRationalQuotient(m.manager, &m.priceWork.Data[0], &m.ThetaP, &m.Delta, &m.PivotRow.Data[q]); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	return nil
}
func (m *RationalDualCoreSolver) BasisChangeAndUpdate() (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.UpdateBetas(); err != nil {
		return false, err
	}
	if err = m.UpdateDAndXB(); err != nil {
		return false, err
	}
	if err = m.assignDualPrimalTheta(); err != nil {
		return false, err
	}
	if err = m.ApplyFlips(); err != nil {
		return false, err
	}
	ok, err = m.UpdateBasisAndX(m.Q, m.P, &m.ThetaP)
	if err != nil {
		return false, err
	}
	if !ok {
		if err = m.InitBetasPrecisely(); err != nil {
			return false, err
		}
		return false, nil
	}
	snapped, e := m.SnapRunawayNonBasicColumn(uint32(m.P))
	if e != nil {
		return false, e
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
func (m *RationalDualCoreSolver) AdvanceOnKnownP() (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	done, e := m.Done()
	if e != nil {
		return false, e
	}
	if done {
		return true, nil
	}
	if err = m.CalculatePivotRowOfBInverse(uint32(m.R)); err != nil {
		return false, err
	}
	if err = m.CalculatePivotRowWhenInverseReady(uint32(m.R)); err != nil {
		return false, err
	}
	ok, err = m.RatioTest()
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
	comparison, e := m.PivotsInColumnAndRowAreDifferent(m.Q, m.P)
	if e != nil {
		return false, e
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
