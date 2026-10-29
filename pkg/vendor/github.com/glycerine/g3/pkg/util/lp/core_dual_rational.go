package lp

// Source: src/util/lp/lp_core_solver_base.hpp:390-444. Concrete mpq/mpq.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (m *RationalCoreSolverBase) DIsNotNegative(j uint32) (bool, error) {
	if err := m.ready(); err != nil {
		return false, err
	}
	if uint64(j) >= uint64(len(m.D.Data)) {
		return false, ErrBounds
	}
	zero := numeral.NewRational()
	c, err := m.compare(&m.D.Data[j], &zero)
	return c >= 0, err
}
func (m *RationalCoreSolverBase) DIsNotPositive(j uint32) (bool, error) {
	if err := m.ready(); err != nil {
		return false, err
	}
	if uint64(j) >= uint64(len(m.D.Data)) {
		return false, ErrBounds
	}
	zero := numeral.NewRational()
	c, err := m.compare(&m.D.Data[j], &zero)
	return c <= 0, err
}
func (m *RationalCoreSolverBase) ColumnIsDualFeasible(j uint32) (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed, ColumnBoxed, ColumnLowBound:
		if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.LowBounds)) {
			return false, ErrBounds
		}
		at, e := m.AtBound(&(*m.X)[j], &(*m.LowBounds)[j])
		if e != nil {
			return false, e
		}
		if at {
			ok, e = m.DIsNotNegative(j)
			if e != nil || ok {
				return ok, e
			}
		}
		if (*m.ColumnTypes)[j] == ColumnLowBound {
			return false, nil
		}
		if uint64(j) >= uint64(len(*m.UpperBounds)) {
			return false, ErrBounds
		}
		at, e = m.AtBound(&(*m.X)[j], &(*m.UpperBounds)[j])
		if e != nil || !at {
			return false, e
		}
		return m.DIsNotPositive(j)
	case ColumnUpperBound:
		if m.Settings.DebugOut != nil {
			if _, err = fmt.Fprintln(m.Settings.DebugOut, "upper_bound type should be switched to low_bound"); err != nil {
				return false, err
			}
		}
		if err = sourceAssertionFailure(); err != nil {
			return false, err
		}
		fallthrough
	case ColumnFree:
		if uint64(j) >= uint64(len(m.D.Data)) {
			return false, ErrBounds
		}
		return m.manager.IsZero(&m.D.Data[j])
	default:
		if m.Settings.DebugOut != nil {
			if _, err = fmt.Fprintf(m.Settings.DebugOut, "column = %d\n", j); err != nil {
				return false, err
			}
			name, e := ColumnTypeString((*m.ColumnTypes)[j])
			if e != nil {
				return false, e
			}
			if _, err = fmt.Fprintf(m.Settings.DebugOut, "unexpected column type = %s\n", name); err != nil {
				return false, err
			}
		}
		return false, sourceAssertionFailure()
	}
}
func (m *RationalCoreSolverBase) SetNonBasicXToCorrectBounds() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	for _, j := range *m.NBasis {
		if uint64(j) >= uint64(len(*m.ColumnTypes)) {
			return ErrBounds
		}
		var bounds *[]numeral.Rational
		switch (*m.ColumnTypes)[j] {
		case ColumnBoxed:
			if uint64(j) >= uint64(len(m.D.Data)) {
				return ErrBounds
			}
			zero := numeral.NewRational()
			c, e := m.compare(&m.D.Data[j], &zero)
			if e != nil {
				return e
			}
			bounds = m.LowBounds
			if c < 0 {
				bounds = m.UpperBounds
			}
		case ColumnLowBound:
			bounds = m.LowBounds
		case ColumnUpperBound:
			bounds = m.UpperBounds
		default:
			continue
		}
		if uint64(j) >= uint64(len(*bounds)) || uint64(j) >= uint64(len(*m.X)) {
			return ErrBounds
		}
		before, e := m.calls.begin(m.manager)
		if e != nil {
			return e
		}
		err = m.manager.Copy(&(*m.X)[j], &(*bounds)[j])
		if err = m.calls.after(m.manager, before, err); err != nil {
			return err
		}
		if sourceDebugEnabled && (*m.ColumnTypes)[j] != ColumnBoxed {
			ok, e := m.ColumnIsDualFeasible(j)
			if e != nil {
				return e
			}
			if !ok {
				return ErrAssertion
			}
		}
	}
	return nil
}
