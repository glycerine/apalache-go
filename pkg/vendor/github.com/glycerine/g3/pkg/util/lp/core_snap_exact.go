package lp

// Source: src/util/lp/lp_core_solver_base.h:346-385; retained mpq/impq.

// SnapColumnToBound preserves an existing endpoint; an interior boxed value
// consumes exactly one source random value. Free and unknown types do nothing.
func (m *PairCoreSolverBase) SnapColumnToBound(j uint32) (changed bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	upper := false
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed, ColumnBoxed:
		same, e := m.exactAtColumnBound(j, false)
		if e != nil {
			return false, e
		}
		if !same {
			same, e = m.exactAtColumnBound(j, true)
			if e != nil {
				return false, e
			}
		}
		if same {
			return false, nil
		}
		if (*m.ColumnTypes)[j] == ColumnBoxed {
			upper = m.Settings.RandomNext()%2 != 1
		}
	case ColumnLowBound, ColumnUpperBound:
		upper = (*m.ColumnTypes)[j] == ColumnUpperBound
		same, e := m.exactAtColumnBound(j, upper)
		if e != nil {
			return false, e
		}
		if same {
			return false, nil
		}
	default:
		return false, nil
	}
	bounds := m.LowBounds
	if upper {
		bounds = m.UpperBounds
	}
	if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*bounds)) {
		return false, ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	err = (*m.X)[j].Copy(m.manager, &(*bounds)[j])
	if err = m.calls.after(m.manager, before, err); err != nil {
		return false, err
	}
	return true, nil
}

func (m *PairCoreSolverBase) SnapNonBasicXToBound() (changed bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	for _, j := range *m.NBasis {
		snapped, e := m.SnapColumnToBound(j)
		if e != nil {
			return changed, e
		}
		// Source puts the call on the left of ||, even after an earlier change.
		changed = snapped || changed
	}
	return changed, nil
}
