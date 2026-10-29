package lp

// Source: src/util/lp/lp_core_solver_base.hpp:868-885; scalar mpq/mpq.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalCoreSolverBase) snapNonBasicXToBoundAndFreeToZeroes() (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	for _, j := range *m.NBasis {
		if uint64(j) >= uint64(len(*m.X)) {
			if err = sourceAssertionFailure(); err != nil {
				return err
			}
			return ErrBounds
		}
		if uint64(j) >= uint64(len(*m.ColumnTypes)) {
			return ErrBounds
		}
		var bound *numeral.Rational
		switch (*m.ColumnTypes)[j] {
		case ColumnFixed, ColumnBoxed, ColumnLowBound:
			if uint64(j) >= uint64(len(*m.LowBounds)) {
				return ErrBounds
			}
			bound = &(*m.LowBounds)[j]
		case ColumnUpperBound:
			if uint64(j) >= uint64(len(*m.UpperBounds)) {
				return ErrBounds
			}
			bound = &(*m.UpperBounds)[j]
		default:
			zero := numeral.NewRational()
			bound = &zero
		}
		if err = m.manager.Copy(&(*m.X)[j], bound); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalCoreSolverBase) SnapNonBasicXToBoundAndFreeToZeroes() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.snapNonBasicXToBoundAndFreeToZeroes()
}
func (m *RationalCoreSolverBase) SnapXNToBoundsAndFreeColumnsToZeroes() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.snapNonBasicXToBoundAndFreeToZeroes(); err != nil {
		return err
	}
	return m.solveAxEqualsBValue()
}
