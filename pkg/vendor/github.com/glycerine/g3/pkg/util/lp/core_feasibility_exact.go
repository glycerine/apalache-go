package lp

// Source: src/util/lp/lp_core_solver_base.h:387-432; mpq/impq.

// MakeColumnFeasible clamps a nonbasic value and returns its exact delta.
// No change leaves delta untouched. The caller owns delta, including a partial
// assignment on error; the core owns source expression temporaries and calls.
func (m *PairCoreSolverBase) MakeColumnFeasible(j uint32, delta *RationalPair) (changed bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if delta == nil || uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	if sourceDebugEnabled {
		if uint64(j) >= uint64(len(*m.Heading)) {
			return false, ErrBounds
		}
		if (*m.Heading)[j] >= 0 {
			return false, ErrAssertion
		}
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	x := &(*m.X)[j]
	var bound *RationalPair
	low := func() (*RationalPair, error) {
		if uint64(j) >= uint64(len(*m.LowBounds)) {
			return nil, ErrBounds
		}
		return &(*m.LowBounds)[j], nil
	}
	upper := func() (*RationalPair, error) {
		if uint64(j) >= uint64(len(*m.UpperBounds)) {
			return nil, ErrBounds
		}
		return &(*m.UpperBounds)[j], nil
	}
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed:
		b, e := low()
		if e != nil {
			return false, e
		}
		if sourceDebugEnabled {
			u, e := upper()
			if e != nil {
				return false, e
			}
			same, e := b.Equal(m.manager, u)
			if e != nil {
				return false, e
			}
			if !same {
				return false, ErrAssertion
			}
		}
		same, e := x.Equal(m.manager, b)
		if e != nil {
			return false, e
		}
		if !same {
			bound = b
		}
	case ColumnBoxed, ColumnLowBound:
		b, e := low()
		if e != nil {
			return false, e
		}
		cmp, e := x.Compare(m.manager, b)
		if e != nil {
			return false, e
		}
		if cmp < 0 {
			bound = b
		} else if (*m.ColumnTypes)[j] == ColumnBoxed {
			u, e := upper()
			if e != nil {
				return false, e
			}
			cmp, e = x.Compare(m.manager, u)
			if e != nil {
				return false, e
			}
			if cmp > 0 {
				bound = u
			}
		}
	case ColumnUpperBound:
		u, e := upper()
		if e != nil {
			return false, e
		}
		cmp, e := x.Compare(m.manager, u)
		if e != nil {
			return false, e
		}
		if cmp > 0 {
			bound = u
		}
	case ColumnFree:
	default:
		if err = sourceAssertionFailure(); err != nil {
			return false, err
		}
	}
	if bound == nil {
		return false, nil
	}
	// Source constructs both rational expression parameters, then the returned
	// pair, then assigns it to delta before replacing x in the next statement.
	m.pivotWork = NewIndexedRationalVector(m.manager, 3)
	m.feasibilityWork = NewIndexedPairVector(m.manager, 1)
	tmp, px, py := &m.pivotWork.Data[0], &m.pivotWork.Data[1], &m.pivotWork.Data[2]
	if err = etaRationalDifference(m.manager, tmp, px, &bound.X, &x.X); err != nil {
		return false, err
	}
	if err = etaRationalDifference(m.manager, tmp, py, &bound.Y, &x.Y); err != nil {
		return false, err
	}
	result := &m.feasibilityWork.Data[0]
	if err = m.manager.Copy(&result.X, px); err != nil {
		return false, err
	}
	if err = m.manager.Copy(&result.Y, py); err != nil {
		return false, err
	}
	if err = delta.Copy(m.manager, result); err != nil {
		return false, err
	}
	if err = m.feasibilityWork.Close(); err != nil {
		return false, err
	}
	m.feasibilityWork = nil
	if err = m.pivotWork.Close(); err != nil {
		return false, err
	}
	m.pivotWork = nil
	if err = x.Copy(m.manager, bound); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateColumnInInfSet maintains the source dense infeasibility membership.
func (m *PairCoreSolverBase) UpdateColumnInInfSet(j uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	feasible, e := m.ColumnIsFeasible(j)
	if e != nil {
		return e
	}
	if feasible {
		m.InfSet.Erase(j)
		return nil
	}
	return m.InfSet.Insert(j)
}
