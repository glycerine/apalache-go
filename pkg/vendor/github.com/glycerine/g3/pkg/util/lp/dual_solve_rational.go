package lp

// Source: src/util/lp/lp_dual_core_solver.hpp:162-204,731-761.
// Concrete retained mpq/mpq pricing, iteration and prepared dual core Solve.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalDualCoreSolver) updateLeavingMaximum() (improved bool, err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	maximum, score := &m.loopValues.Data[0], &m.loopValues.Data[1]
	// Source rational greater-than reverses the less-than operands.
	cmp, e := m.manager.Compare(maximum, score)
	if e != nil {
		return false, e
	}
	if cmp < 0 {
		if err = m.manager.Copy(maximum, score); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
func (m *RationalDualCoreSolver) finishPricingLoop() error {
	if err := m.loopValues.Close(); err != nil {
		return err
	}
	m.loopValues = nil
	return nil
}

// The budget counts record-breaking scores. The loop-local score dies after
// each visit; the owning maximum stays live through actual advancement.
func (m *RationalDualCoreSolver) PricingLoop(numberOfRowsToTry, offset uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.loopValues != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	m.R = -1
	m.loopValues = NewIndexedRationalVector(m.manager, 2)
	rows := m.A.RowCount()
	if rows == 0 || offset >= rows {
		return ErrBounds
	}
	left, i := numberOfRowsToTry, offset
	for {
		if !m.ForbiddenRows[i] {
			if err = m.PricingForRow(i, &m.loopValues.Data[1]); err != nil {
				return err
			}
			improved, e := m.updateLeavingMaximum()
			if e != nil {
				return e
			}
			if improved {
				m.R = int32(i)
				if left > 0 {
					left--
				}
			}
			if err = m.manager.Del(&m.loopValues.Data[1]); err != nil {
				return err
			}
			m.loopValues.Data[1] = numeral.NewRational()
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
		return m.finishPricingLoop()
	}
	if uint64(m.R) >= uint64(len(*m.Basis)) {
		return ErrBounds
	}
	m.P = int32((*m.Basis)[m.R])
	if err = m.GetDelta(&m.Delta); err != nil {
		return err
	}
	ok, e := m.AdvanceOnKnownP()
	if e != nil {
		return e
	}
	if ok {
		m.ForbiddenRows = nil
		return m.finishPricingLoop()
	}
	if m.Status == StatusFloatingPointError {
		return m.finishPricingLoop()
	}
	m.Status = StatusUnstable
	if m.ForbiddenRows == nil {
		m.ForbiddenRows = make(map[uint32]bool)
	}
	m.ForbiddenRows[uint32(m.R)] = true
	return m.finishPricingLoop()
}
func (m *RationalDualCoreSolver) OneIteration() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	n, e := m.GetNumberOfRowsToTryForLeaving()
	if e != nil {
		return e
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
		ok, e := m.ProblemIsDualFeasible()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
	}
	return nil
}

// Solve expects prepared X, reduced costs and betas, exactly as the C++ core.
// Outer primal/dual construction, stage transitions and scaling are separate.
func (m *RationalDualCoreSolver) Solve() (count uint32, err error) {
	if err = m.ready(); err != nil {
		return m.TotalIterations, err
	}
	defer func() { m.stopped = err != nil; count = m.TotalIterations }()
	if sourceDebugEnabled {
		ok, e := m.DIsCorrect()
		if e != nil {
			return 0, e
		}
		if !ok {
			return 0, ErrAssertion
		}
		ok, e = m.ProblemIsDualFeasible()
		if e != nil {
			return 0, e
		}
		if !ok {
			return 0, ErrAssertion
		}
		ok, e = m.BasisHeadingIsCorrect()
		if e != nil {
			return 0, e
		}
		if !ok {
			return 0, ErrAssertion
		}
	}
	m.TotalIterations = 0
	m.IterationsNoCostGrowth = 0
	for {
		prefix := ""
		over, e := m.PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(&prefix, m.Settings.MessageOut)
		if e != nil {
			return 0, e
		}
		if over {
			return 0, nil
		}
		if err = m.OneIteration(); err != nil {
			return 0, err
		}
		if m.Status == StatusFloatingPointError || m.Status == StatusDualUnbounded || m.Status == StatusOptimal || m.IterationsNoCostGrowth > m.Settings.MaxIterationsWithoutImprovements || m.TotalIterations > m.Settings.MaxTotalIterations {
			break
		}
	}
	return 0, nil
}
