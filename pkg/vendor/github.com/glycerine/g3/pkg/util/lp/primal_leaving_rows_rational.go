package lp

// Source: src/util/lp/lp_primal_core_solver.h:451-481.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalPrimalCoreSolver) FindLeavingTableauRows(out *numeral.Rational) (leaving int32, err error) {
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
	var bound *numeral.Rational
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
		err = m.manager.Copy(out, bound)
	} else {
		err = m.manager.SetInt32(out, 0)
	}
	return leaving, err
}
