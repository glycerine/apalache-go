package lp

// Source: src/util/lp/lp_core_solver_base.hpp:893-914, mpq/impq.

func (m *PairCoreSolverBase) exactAtColumnBound(j uint32, upper bool) (bool, error) {
	bounds := m.LowBounds
	if upper {
		bounds = m.UpperBounds
	}
	if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*bounds)) {
		return false, ErrBounds
	}
	return m.AtBound(&(*m.X)[j], &(*bounds)[j])
}

func (m *PairCoreSolverBase) GetNonBasicColumnValuePosition(j uint32) (position NonBasicColumnValuePosition, err error) {
	if err = m.ready(); err != nil {
		return PositionAtLowBound, err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return PositionAtLowBound, ErrBounds
	}
	kind := (*m.ColumnTypes)[j]
	if kind == ColumnFree {
		return PositionFreeOfBounds, nil
	}
	if kind == ColumnUpperBound {
		at, e := m.exactAtColumnBound(j, true)
		if e != nil {
			return PositionAtLowBound, e
		}
		if at {
			return PositionAtUpperBound, nil
		}
		return PositionNotAtBound, nil
	}
	if kind != ColumnFixed && kind != ColumnLowBound && kind != ColumnBoxed {
		return PositionAtLowBound, sourceAssertionFailure()
	}
	at, e := m.exactAtColumnBound(j, false)
	if e != nil {
		return PositionAtLowBound, e
	}
	if at {
		if kind == ColumnFixed {
			return PositionAtFixed, nil
		}
		return PositionAtLowBound, nil
	}
	if kind == ColumnBoxed {
		at, e = m.exactAtColumnBound(j, true)
		if e != nil {
			return PositionAtLowBound, e
		}
		if at {
			return PositionAtUpperBound, nil
		}
	}
	return PositionNotAtBound, nil
}
