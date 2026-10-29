package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:771-790, precise specialization.

// AdvanceOnEnteringPrecise solves the actual entering column before creating
// its local step. The local stays owned until all children finish, including
// stopped arithmetic borrowing it. Unbounded is the source status transition;
// there is no floating reduced-cost refresh or second observation in this path.
func (m *RationalPrimalCoreSolver) AdvanceOnEnteringPrecise(entering int32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.enteringStep != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled && entering < 0 {
		return ErrAssertion
	}
	if err = m.SolveBd(uint32(entering)); err != nil {
		return err
	}
	m.enteringStep = NewIndexedRationalVector(m.manager, 1)
	leaving, e := m.FindLeavingAndTPrecise(uint32(entering), &m.enteringStep.Data[0])
	if e != nil {
		return e
	}
	if leaving == -1 {
		m.Status = StatusUnbounded
	} else {
		if err = m.AdvanceOnEnteringAndLeaving(entering, int32(leaving), &m.enteringStep.Data[0]); err != nil {
			return err
		}
	}
	if err = m.enteringStep.Close(); err != nil {
		return err
	}
	m.enteringStep = nil
	return nil
}

func (m *RationalPrimalCoreSolver) AdvanceOnEntering(entering int32) error {
	return m.AdvanceOnEnteringPrecise(entering)
}
