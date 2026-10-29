package lp

// Source: src/util/lp/lp_core_solver_base.hpp:460-477,790-796,831-836.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalCoreSolverBase) RsMinusAnX(rs []numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.rsMinusAnXValue(rs)
}
func (m *RationalCoreSolverBase) rsMinusAnXValue(rs []numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	for row := m.A.RowCount(); row > 0; {
		row--
		if uint64(row) >= uint64(len(rs)) || uint64(row) >= uint64(len(*m.B)) {
			return ErrBounds
		}
		if err = m.manager.Copy(&rs[row], &(*m.B)[row]); err != nil {
			return err
		}
		for k := range m.A.Rows[row] {
			c := &m.A.Rows[row][k]
			j := c.Column
			if uint64(j) >= uint64(len(*m.Heading)) {
				return ErrBounds
			}
			if (*m.Heading)[j] < 0 {
				if uint64(j) >= uint64(len(*m.X)) {
					return ErrBounds
				}
				if err = m.subtractCoreValueProduct(&rs[row], &(*m.X)[j], &c.Value); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func (m *RationalCoreSolverBase) CopyRsToXB(rs []numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.copyRsToXBValue(rs)
}
func (m *RationalCoreSolverBase) copyRsToXBValue(rs []numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(*m.Basis)) {
			return ErrBounds
		}
		j := (*m.Basis)[i]
		if uint64(j) >= uint64(len(*m.X)) || uint64(i) >= uint64(len(rs)) {
			return ErrBounds
		}
		if err = m.manager.Copy(&(*m.X)[j], &rs[i]); err != nil {
			return err
		}
	}
	return nil
}

func (m *RationalCoreSolverBase) SolveAxEqualsB() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.solveAxEqualsBValue()
}
func (m *RationalCoreSolverBase) solveAxEqualsBValue() (err error) {
	m.reconstructionWork = NewIndexedRationalVector(m.manager, m.A.RowCount())
	if err = m.rsMinusAnXValue(m.reconstructionWork.Data); err != nil {
		return err
	}
	// This is the source precise branch: one solve, with no refinement pass.
	if m.Factorization == nil {
		return ErrBounds
	}
	if err = m.Factorization.SolveBy(m.reconstructionWork.Data); err != nil {
		return err
	}
	if err = m.copyRsToXBValue(m.reconstructionWork.Data); err != nil {
		return err
	}
	if err = m.reconstructionWork.Close(); err != nil {
		return err
	}
	m.reconstructionWork = nil
	return nil
}
