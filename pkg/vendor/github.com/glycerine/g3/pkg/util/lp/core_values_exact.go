package lp

// Source: src/util/lp/lp_core_solver_base.hpp:261-263,334-348,758-766;
// numeric_pair.h:141-143,164-173.

import "github.com/glycerine/g3/pkg/util/numeral"

// subtractCoreValueProduct evaluates the whole scalar*delta pair before publishing
// the compound subtraction. delta and scalar remain live references, including
// when delta aliases a value changed by an earlier row or the entering update.
func (m *PairCoreSolverBase) subtractCoreValueProduct(dst, delta *RationalPair, scalar *numeral.Rational) (err error) {
	m.valueScalars = NewIndexedRationalVector(m.manager, 3)
	m.valueWork = NewIndexedPairVector(m.manager, 1)
	tmp, x, y := &m.valueScalars.Data[0], &m.valueScalars.Data[1], &m.valueScalars.Data[2]
	if err = etaRationalProduct(m.manager, tmp, x, scalar, &delta.X); err != nil {
		return err
	}
	if err = etaRationalProduct(m.manager, tmp, y, scalar, &delta.Y); err != nil {
		return err
	}
	product := &m.valueWork.Data[0]
	if err = m.manager.Copy(&product.X, x); err != nil {
		return err
	}
	if err = m.manager.Copy(&product.Y, y); err != nil {
		return err
	}
	// The constructor's scalar expression temporaries end before operator-=.
	if err = m.valueScalars.Close(); err != nil {
		return err
	}
	m.valueScalars = nil
	if err = dst.Sub(m.manager, dst, product); err != nil {
		return err
	}
	if err = m.valueWork.Close(); err != nil {
		return err
	}
	m.valueWork = nil
	return nil
}

// UpdateX publishes the entering increment first, then each basic subtraction
// in its source traversal order. The precise path never writes CopyXB.
func (m *PairCoreSolverBase) UpdateX(entering uint32, delta *RationalPair) (err error) {
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
	if err = (*m.X)[entering].Add(m.manager, &(*m.X)[entering], delta); err != nil {
		return err
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
			value, e := m.A.ColumnValue(c)
			if e != nil {
				return e
			}
			if err = m.subtractCoreValueProduct(&(*m.X)[j], delta, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// RestoreX uses the stored backup even in precise mode. It is not an inverse of
// UpdateX unless the caller has populated CopyXB with the intended old values.
func (m *PairCoreSolverBase) RestoreX(entering uint32, t *RationalPair) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if t == nil {
		return ErrBounds
	}
	zero, e := t.IsZero(m.manager)
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
	if err = (*m.X)[entering].Sub(m.manager, &(*m.X)[entering], t); err != nil {
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
		if err = (*m.X)[j].Copy(m.manager, &m.CopyXB.Data[i]); err != nil {
			return err
		}
	}
	return nil
}

// The source precise branch returns before reading any indexed row or RHS.
func (m *PairCoreSolverBase) AMultXIsOffOnIndex(index []uint32) (off bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled && len(*m.X) != int(m.A.ColumnCount()) {
		return false, ErrAssertion
	}
	return false, nil
}
