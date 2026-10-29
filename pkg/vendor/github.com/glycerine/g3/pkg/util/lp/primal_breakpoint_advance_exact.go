package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:274-303.

func (m *PairPrimalCoreSolver) copyBreakpointSlope(entering uint32) (err error) {
	if uint64(entering) >= uint64(len(m.D.Data)) {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.breakpointSlope = NewIndexedRationalVector(m.manager, 1)
	return m.manager.Copy(&m.breakpointSlope.Data[0], &m.D.Data[entering])
}

// Evaluate the source full expression after the slope update. The capture ends
// before the next queue operation; the queue keeps its own comparison receipts.
func (m *PairPrimalCoreSolver) breakpointSlopeIncreasing() (increasing bool, err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 4)
	direction, product, threshold, temporary := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2], &m.work.Data[3]
	if err = m.manager.SetInt32(direction, m.SignOfEnteringDelta); err != nil {
		return false, err
	}
	if err = etaRationalProduct(m.manager, temporary, product, &m.breakpointSlope.Data[0], direction); err != nil {
		return false, err
	}
	if err = m.manager.Copy(threshold, &m.EpsilonOfReducedCost); err != nil {
		return false, err
	}
	if err = m.manager.Neg(threshold); err != nil {
		return false, err
	}
	cmp, e := m.manager.Compare(product, threshold)
	if e != nil {
		return false, e
	}
	if err = m.work.Close(); err != nil {
		return false, err
	}
	m.work = nil
	return cmp > 0, nil
}

func (m *PairPrimalCoreSolver) finishBreakpointAdvance(b *PairBreakpoint, t *RationalPair) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	if err = t.Copy(m.manager, b.Delta); err != nil {
		return err
	}
	if err = m.breakpointSlope.Close(); err != nil {
		return err
	}
	m.breakpointSlope = nil
	return nil
}

// AdvanceOnSortedBreakpoints consumes priorities in source queue order. Records
// remain owned by the primal, including after a partially completed dequeue.
func (m *PairPrimalCoreSolver) AdvanceOnSortedBreakpoints(entering uint32, t *RationalPair) (leaving int, err error) {
	leaving = -1
	if err = m.ready(); err != nil {
		return
	}
	defer func() { m.stopped = err != nil }()
	if t == nil {
		return -1, ErrBounds
	}
	if err = m.copyBreakpointSlope(entering); err != nil {
		return
	}
	if sourceDebugEnabled && m.BreakpointQueue.Size() == 0 {
		return -1, ErrAssertion
	}
	var last *PairBreakpoint
	for m.BreakpointQueue.Size() != 0 {
		bi, e := m.BreakpointQueue.Dequeue()
		if e != nil {
			return -1, e
		}
		if uint64(bi) >= uint64(len(m.Breakpoints)) || m.Breakpoints[bi] == nil {
			return -1, ErrBounds
		}
		b := m.Breakpoints[bi]
		if err = m.changeSlopeOnBreakpointValue(entering, b, &m.breakpointSlope.Data[0]); err != nil {
			return
		}
		last = b
		increasing, e := m.breakpointSlopeIncreasing()
		if e != nil {
			return -1, e
		}
		if increasing {
			break
		}
		zero, e := m.manager.IsZero(&m.breakpointSlope.Data[0])
		if e != nil {
			return -1, e
		}
		if zero && m.Settings.RandomNext()%2 == 0 {
			break
		}
	}
	if last == nil {
		if sourceDebugEnabled {
			return -1, ErrAssertion
		}
		return -1, ErrBounds
	}
	if err = m.finishBreakpointAdvance(last, t); err != nil {
		return
	}
	return int(last.Column), nil
}

func (m *PairPrimalCoreSolver) FindLeavingAndTWithBreakpoints(entering uint32, t *RationalPair) (leaving int, err error) {
	leaving = -1
	if err = m.ready(); err != nil {
		return
	}
	defer func() { m.stopped = err != nil }()
	// The upstream wrapper asserts precise()==false, even though its precise
	// leaving selector can reach this method. Native builds erase the assertion.
	if sourceDebugEnabled {
		return -1, ErrAssertion
	}
	if err = m.FillBreakpointsArray(entering); err != nil {
		return
	}
	return m.AdvanceOnSortedBreakpoints(entering, t)
}
