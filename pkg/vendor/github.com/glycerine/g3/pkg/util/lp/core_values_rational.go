package lp

// Source: src/util/lp/lp_core_solver_base.hpp:261-263,334-348,758-766;
// lu.h:48-57 and lp_core_solver_base.h:153-155; retained scalar mpq/mpq.

import "github.com/glycerine/g3/pkg/util/numeral"

// Source rational binary multiplication copies the left operand, updates that
// temporary, and copies the returned reference before the compound subtraction.
func (m *RationalCoreSolverBase) subtractCoreValueProduct(dst, left, right *numeral.Rational) error {
	m.valueWork = NewIndexedRationalVector(m.manager, 2)
	if err := etaRationalProduct(m.manager, &m.valueWork.Data[0], &m.valueWork.Data[1], left, right); err != nil {
		return err
	}
	if err := m.manager.Sub(dst, &m.valueWork.Data[1], dst); err != nil {
		return err
	}
	if err := m.valueWork.Close(); err != nil {
		return err
	}
	m.valueWork = nil
	return nil
}

// UpdateX keeps delta a live reference. An entering/earlier basic update can
// therefore change the delta subsequently read by the source loop.
func (m *RationalCoreSolverBase) UpdateX(entering uint32, delta *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if delta == nil || uint64(entering) >= uint64(len(*m.X)) {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	if err = m.manager.Add(&(*m.X)[entering], delta, &(*m.X)[entering]); err != nil {
		return err
	}
	if m.Settings == nil {
		return ErrBounds
	}
	if !m.Settings.UseTableau() {
		for _, i := range m.Ed.Index {
			if uint64(i) >= uint64(len(*m.Basis)) {
				return ErrBounds
			}
			j := (*m.Basis)[i]
			if uint64(j) >= uint64(len(*m.X)) || uint64(i) >= uint64(len(m.Ed.Data)) {
				return ErrBounds
			}
			if err = m.subtractCoreValueProduct(&(*m.X)[j], delta, &m.Ed.Data[i]); err != nil {
				return err
			}
		}
	} else {
		if uint64(entering) >= uint64(len(m.A.Columns)) {
			return ErrBounds
		}
		for _, c := range m.A.Columns[entering] {
			if uint64(c.Row) >= uint64(len(*m.Basis)) {
				return ErrBounds
			}
			j := (*m.Basis)[c.Row]
			if uint64(j) >= uint64(len(*m.X)) {
				return ErrBounds
			}
			v, e := m.A.ColumnValue(c)
			if e != nil {
				return e
			}
			if err = m.subtractCoreValueProduct(&(*m.X)[j], delta, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// Precise UpdateX does not save CopyXB. RestoreX deliberately uses the existing
// backup, so it is an inverse only when the caller installed that backup.
func (m *RationalCoreSolverBase) RestoreX(entering uint32, t *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if t == nil {
		return ErrBounds
	}
	zero, e := m.manager.IsZero(t)
	if e != nil {
		return e
	}
	if zero {
		return nil
	}
	if uint64(entering) >= uint64(len(*m.X)) {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	if err = m.manager.Sub(&(*m.X)[entering], t, &(*m.X)[entering]); err != nil {
		return err
	}
	for _, i := range m.Ed.Index {
		if uint64(i) >= uint64(len(*m.Basis)) {
			return ErrBounds
		}
		j := (*m.Basis)[i]
		if uint64(j) >= uint64(len(*m.X)) || uint64(i) >= uint64(len(m.CopyXB.Data)) {
			return ErrBounds
		}
		if err = m.manager.Copy(&(*m.X)[j], &m.CopyXB.Data[i]); err != nil {
			return err
		}
	}
	return nil
}

// The precise indexed residual query returns before reading any row or RHS.
func (m *RationalCoreSolverBase) AMultXIsOffOnIndex(index []uint32) (off bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled && len(*m.X) != int(m.A.ColumnCount()) {
		return false, ErrAssertion
	}
	return false, nil
}

// GetCost is the source ascending dot_product of Costs and X. The accumulator
// and product temporaries remain owned by the core on failure; caller out is
// assigned only on return and must outlive Close after a failed final copy.
func (m *RationalCoreSolverBase) GetCost(out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if len(*m.Costs) != len(*m.X) {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.costWork = NewIndexedRationalVector(m.manager, 3)
	w := m.costWork
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	for j := range *m.Costs {
		if j >= len(*m.X) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, &w.Data[1], &w.Data[2], &(*m.Costs)[j], &(*m.X)[j]); err != nil {
			return err
		}
		if err = m.manager.Add(&w.Data[0], &w.Data[2], &w.Data[0]); err != nil {
			return err
		}
		if err = m.manager.Del(&w.Data[2]); err != nil {
			return err
		}
		w.Data[2] = numeral.NewRational()
	}
	if err = m.manager.Copy(out, &w.Data[0]); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	m.costWork = nil
	return nil
}
