package lp

// Source: src/util/lp/lp_primal_core_solver.h:446-449,483-511.

func (m *PairPrimalCoreSolver) closeRowIterationLocals() error {
	if m.rowIterationTheta != nil {
		if err := m.rowIterationTheta.Close(); err != nil {
			return err
		}
		m.rowIterationTheta = nil
	}
	if m.rowIterationCoefficient != nil {
		if err := m.rowIterationCoefficient.Close(); err != nil {
			return err
		}
		m.rowIterationCoefficient = nil
	}
	if m.rowIterationBound != nil {
		if err := m.rowIterationBound.Close(); err != nil {
			return err
		}
		m.rowIterationBound = nil
	}
	return nil
}

// AdvanceOnEnteringAndLeavingTableauRows retains the ignored pivot result and
// refreshes the entering column's membership only after basis publication.
func (m *PairPrimalCoreSolver) AdvanceOnEnteringAndLeavingTableauRows(entering, leaving int32, theta *RationalPair) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if _, err = m.UpdateBasisAndXTableau(entering, leaving, theta); err != nil {
		return err
	}
	return m.UpdateColumnInInfSet(uint32(entering))
}

func (m *PairPrimalCoreSolver) buildRowIterationTheta(leaving uint32) (err error) {
	if uint64(leaving) >= uint64(len(*m.X)) {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.pairWork = NewIndexedPairVector(m.manager, 1)
	m.rowIterationTheta = NewIndexedPairVector(m.manager, 1)
	if err = m.pairWork.Data[0].Sub(m.manager, &(*m.X)[leaving], &m.rowIterationBound.Data[0]); err != nil {
		return err
	}
	if err = m.rowIterationTheta.Data[0].Divide(m.manager, &m.pairWork.Data[0], &m.rowIterationCoefficient.Data[0]); err != nil {
		return err
	}
	if err = m.pairWork.Close(); err != nil {
		return err
	}
	m.pairWork = nil
	return nil
}
func (m *PairPrimalCoreSolver) rowIterationLeavingEqualsBound(leaving uint32) (ok bool, err error) {
	if uint64(leaving) >= uint64(len(*m.X)) {
		return false, ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return (*m.X)[leaving].Equal(m.manager, &m.rowIterationBound.Data[0])
}

func (m *PairPrimalCoreSolver) OneIterationTableauRows() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.rowIterationBound != nil || m.rowIterationCoefficient != nil || m.rowIterationTheta != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	m.rowIterationBound = NewIndexedPairVector(m.manager, 1)
	leaving, e := m.FindLeavingTableauRows(&m.rowIterationBound.Data[0])
	if e != nil {
		return e
	}
	if leaving == -1 {
		m.Status = StatusOptimal
		return m.closeRowIterationLocals()
	}
	lv := uint32(leaving)
	if !m.BlandModeTableau {
		if m.LeftBasisTableau.Contains(lv) {
			m.LeftBasisRepeated++
			if m.LeftBasisRepeated > m.BlandModeThreshold {
				m.BlandModeTableau = true
			}
		} else if err = m.LeftBasisTableau.Insert(lv); err != nil {
			return err
		}
	}
	m.rowIterationCoefficient = NewIndexedRationalVector(m.manager, 1)
	if uint64(lv) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	entering, e := m.FindBeneficialColumnInRowTableauRows((*m.Heading)[lv], &m.rowIterationCoefficient.Data[0])
	if e != nil {
		return e
	}
	if entering == -1 {
		m.Status = StatusInfeasible
		return m.closeRowIterationLocals()
	}
	if err = m.buildRowIterationTheta(lv); err != nil {
		return err
	}
	if err = m.AdvanceOnEnteringAndLeavingTableauRows(entering, leaving, &m.rowIterationTheta.Data[0]); err != nil {
		return err
	}
	if sourceDebugEnabled {
		same, e := m.rowIterationLeavingEqualsBound(lv)
		if e != nil {
			return e
		}
		if !same {
			return ErrAssertion
		}
	}
	if m.CurrentXIsFeasible() {
		m.Status = StatusOptimal
	}
	return m.closeRowIterationLocals()
}
