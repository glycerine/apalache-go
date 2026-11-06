package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:353-409;
// lp_primal_core_solver.h:337-344,452-482.

// LimitTheta preserves std::min's argument order and assignment prefix. The
// caller owns all pairs, including a partially assigned theta on failure.
func (m *PairPrimalCoreSolver) LimitTheta(lim, theta *RationalPair, unlimited *bool) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if lim == nil || theta == nil || unlimited == nil {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return m.limitThetaValue(lim, theta, unlimited)
}

// The same primal owner calls this while its ratio expression remains live.
func (m *PairPrimalCoreSolver) limitThetaValue(lim, theta *RationalPair, unlimited *bool) (err error) {
	if *unlimited {
		if err = theta.Copy(m.manager, lim); err != nil {
			return err
		}
		*unlimited = false
		return nil
	}
	// std::min(lim, theta) compares theta < lim, choosing lim on equality.
	cmp, e := theta.Compare(m.manager, lim)
	if e != nil {
		return e
	}
	chosen := lim
	if cmp < 0 {
		chosen = theta
	}
	return theta.Copy(m.manager, chosen)
}

func (m *PairPrimalCoreSolver) assignBoundStep(j uint32, out *RationalPair, positive bool) (err error) {
	if out == nil || uint64(j) >= uint64(len(*m.X)) {
		return ErrBounds
	}
	bounds := m.LowBounds
	if positive {
		bounds = m.UpperBounds
	}
	if uint64(j) >= uint64(len(*bounds)) {
		return ErrBounds
	}
	left, right := &(*m.X)[j], &(*bounds)[j]
	if positive {
		left, right = right, left
	}
	m.work = NewIndexedRationalVector(m.manager, 3)
	m.pairWork = NewIndexedPairVector(m.manager, 1)
	tmp, px, py := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
	if err = etaRationalDifference(m.manager, tmp, px, &left.X, &right.X); err != nil {
		return err
	}
	if err = etaRationalDifference(m.manager, tmp, py, &left.Y, &right.Y); err != nil {
		return err
	}
	result := &m.pairWork.Data[0]
	if err = m.manager.Copy(&result.X, px); err != nil {
		return err
	}
	if err = m.manager.Copy(&result.Y, py); err != nil {
		return err
	}
	if err = out.Copy(m.manager, result); err != nil {
		return err
	}
	if err = m.pairWork.Close(); err != nil {
		return err
	}
	m.pairWork = nil
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

// TryJumpToAnotherBoundOnEntering publishes t even when theta rejects the
// flip. Unlimited is read-only; theta and t may alias as in the source.
func (m *PairPrimalCoreSolver) TryJumpToAnotherBoundOnEntering(j uint32, theta, t *RationalPair, unlimited bool) (jump bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.tryJumpToAnotherBoundOnEnteringValue(j, theta, t, unlimited)
}

func (m *PairPrimalCoreSolver) tryJumpToAnotherBoundOnEnteringValue(j uint32, theta, t *RationalPair, unlimited bool) (jump bool, err error) {
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	kind := (*m.ColumnTypes)[j]
	positive := m.SignOfEnteringDelta > 0
	if kind != ColumnBoxed && !(kind == ColumnUpperBound && positive) && !(kind == ColumnLowBound && m.SignOfEnteringDelta < 0) {
		return false, nil
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	if err = m.assignBoundStep(j, t, positive); err != nil {
		return false, err
	}
	jump = unlimited
	if !jump {
		if theta == nil {
			return false, ErrBounds
		}
		cmp, e := t.Compare(m.manager, theta)
		if e != nil {
			return false, e
		}
		jump = cmp <= 0
	}
	if jump && sourceDebugEnabled {
		zero := NewRationalPair()
		cmp, e := t.Compare(m.manager, &zero)
		if e != nil {
			return false, e
		}
		if cmp < 0 {
			return false, ErrAssertion
		}
	}
	return jump, nil
}

// TryJumpToAnotherBoundOnEnteringUnlimited is the distinct source path: only
// boxed columns can flip, and it has no nonnegative-step assertion.
func (m *PairPrimalCoreSolver) TryJumpToAnotherBoundOnEnteringUnlimited(j uint32, t *RationalPair) (jump bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.tryJumpToAnotherBoundOnEnteringUnlimitedValue(j, t)
}

func (m *PairPrimalCoreSolver) tryJumpToAnotherBoundOnEnteringUnlimitedValue(j uint32, t *RationalPair) (jump bool, err error) {
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	if (*m.ColumnTypes)[j] != ColumnBoxed {
		return false, nil
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	if err = m.assignBoundStep(j, t, m.SignOfEnteringDelta > 0); err != nil {
		return false, err
	}
	return true, nil
}

func (m *PairPrimalCoreSolver) FindLeavingTableauRows(out *RationalPair) (leaving int32, err error) {
	if err = m.ready(); err != nil {
		return -1, err
	}
	defer func() { m.stopped = err != nil }()
	leaving = -1
	for _, j := range m.InfSet.Indices() {
		if j < uint32(leaving) {
			leaving = int32(j)
		}
	}
	if leaving == -1 {
		return -1, nil
	}
	j := uint32(leaving)
	if sourceDebugEnabled {
		feasible, e := m.ColumnIsFeasible(j)
		if e != nil {
			return -1, e
		}
		if feasible {
			return -1, ErrAssertion
		}
	}
	if out == nil || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return -1, ErrBounds
	}
	var bound *RationalPair
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed, ColumnUpperBound:
		if uint64(j) >= uint64(len(*m.UpperBounds)) {
			return -1, ErrBounds
		}
		bound = &(*m.UpperBounds)[j]
	case ColumnLowBound:
		if uint64(j) >= uint64(len(*m.LowBounds)) {
			return -1, ErrBounds
		}
		bound = &(*m.LowBounds)[j]
	case ColumnBoxed:
		if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.UpperBounds)) {
			return -1, ErrBounds
		}
		above, e := m.AboveBound(&(*m.X)[j], &(*m.UpperBounds)[j])
		if e != nil {
			return -1, e
		}
		if above {
			bound = &(*m.UpperBounds)[j]
		} else {
			if uint64(j) >= uint64(len(*m.LowBounds)) {
				return -1, ErrBounds
			}
			bound = &(*m.LowBounds)[j]
		}
	default:
		if err = sourceAssertionFailure(); err != nil {
			return -1, err
		}
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return -1, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	if bound != nil {
		err = out.Copy(m.manager, bound)
	} else {
		err = m.manager.SetInt32(&out.X, 0)
		if err == nil {
			err = m.manager.SetInt32(&out.Y, 0)
		}
	}
	return leaving, err
}
