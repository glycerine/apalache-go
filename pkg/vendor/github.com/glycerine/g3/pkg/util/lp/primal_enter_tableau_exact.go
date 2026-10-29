package lp

// Source: src/util/lp/lp_primal_core_solver_tableau.h:23-41.

// AdvanceOnEnteringTableau constructs its step before scanning the stored
// entering column. The local remains alive through every stopped child borrow.
func (m *PairPrimalCoreSolver) AdvanceOnEnteringTableau(entering int32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.enteringStep != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	m.enteringStep = NewIndexedPairVector(m.manager, 1)
	leaving, e := m.FindLeavingAndTTableau(uint32(entering), &m.enteringStep.Data[0])
	if e != nil {
		return e
	}
	if leaving == -1 {
		m.Status = StatusUnbounded
	} else {
		if err = m.AdvanceOnEnteringAndLeavingTableau(entering, int32(leaving), &m.enteringStep.Data[0]); err != nil {
			return err
		}
	}
	if err = m.enteringStep.Close(); err != nil {
		return err
	}
	m.enteringStep = nil
	return nil
}

func (m *PairPrimalCoreSolver) OneIterationTableau() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	entering, e := m.ChooseEnteringColumnTableau()
	if e != nil {
		return e
	}
	if entering == -1 {
		err = m.DecideOnStatusWhenCannotFindEntering()
	} else {
		err = m.AdvanceOnEnteringTableau(entering)
	}
	if err != nil {
		return err
	}
	if sourceDebugEnabled {
		correct, e := m.InfSetIsCorrect()
		if e != nil {
			return e
		}
		if !correct {
			return ErrAssertion
		}
	}
	return nil
}
