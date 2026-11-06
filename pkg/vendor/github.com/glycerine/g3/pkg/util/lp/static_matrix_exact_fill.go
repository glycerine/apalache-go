package lp

// Source: src/util/lp/static_matrix.h:310-358 and init_lar_solver.h:146-148.

import "github.com/glycerine/g3/pkg/util/numeral"

// FillLastRowWithPivoting implements the retained precise term-with-basis
// instantiation. WorkVector is persistent source workspace; lexical values and
// matrix arithmetic calls remain with this matrix on error. The borrowed
// iterator owns its output-copy receipts and must be closed by its caller.
func (m *RationalStaticMatrix) FillLastRowWithPivoting(it *LARTermIterator, basisHeading []int32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if it.term.manager != m.manager {
		return ErrExactVectorOwner
	}
	m.work = NewIndexedRationalVector(m.manager, 6)
	defer func() {
		if err == nil {
			err = m.work.Close()
			if err == nil {
				m.work = nil
			}
		}
	}()
	if len(m.Rows) == 0 {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if m.WorkVector == nil {
		m.WorkVector = NewIndexedRationalVector(m.manager, 0)
	}
	if err = m.WorkVector.Resize(m.ColumnCount()); err != nil {
		return err
	}
	a, negative := &m.work.Data[0], &m.work.Data[1]
	for {
		j, ok, e := it.Next(a)
		if e != nil {
			return e
		}
		if !ok {
			break
		}
		// Capture only this matrix's numeric calls, not borrowed iterator or
		// WorkVector calls. Their owning copy receipts remain with those objects.
		b, e := m.calls.begin(m.manager)
		if e != nil {
			return e
		}
		e = m.manager.Copy(negative, a)
		if e == nil {
			e = m.manager.Neg(negative)
		}
		if err = m.calls.after(m.manager, b, e); err != nil {
			return err
		}
		if err = m.WorkVector.SetValue(negative, j); err != nil {
			return err
		}
		if err = m.manager.Del(negative); err != nil {
			return err
		}
		*negative = numeral.NewRational()
	}
	if err = it.Reset(); err != nil {
		return err
	}
	for {
		j, ok, e := it.NextIndex()
		if e != nil {
			return e
		}
		if !ok {
			break
		}
		if uint64(j) >= uint64(len(basisHeading)) {
			return ErrBounds
		}
		row := basisHeading[j]
		if row < 0 {
			continue
		}
		if uint64(j) >= uint64(len(m.WorkVector.Data)) {
			return ErrBounds
		}
		alpha := &m.WorkVector.Data[j]
		zero, e := m.manager.IsZero(alpha)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		if int64(row) >= int64(len(m.Rows)) {
			return ErrBounds
		}
		for k := range m.Rows[row] {
			c := &m.Rows[row][k]
			if c.Column == j {
				continue
			}
			if uint64(c.Column) >= uint64(len(m.WorkVector.Data)) {
				return ErrBounds
			}
			value := &m.WorkVector.Data[c.Column]
			wasZero, e := m.manager.IsZero(value)
			if e != nil {
				return e
			}
			b, e := m.calls.begin(m.manager)
			if e != nil {
				return e
			}
			e = etaRationalProduct(m.manager, &m.work.Data[2], &m.work.Data[3], alpha, &c.Value)
			if e == nil {
				e = m.manager.Sub(value, &m.work.Data[3], value)
			}
			if err = m.calls.after(m.manager, b, e); err != nil {
				return err
			}
			if wasZero {
				// Source appends even if the new result is an exact stored zero.
				m.WorkVector.Index = append(m.WorkVector.Index, c.Column)
			} else {
				zero, e = m.manager.IsZero(value)
				if e != nil {
					return e
				}
				if zero {
					m.WorkVector.EraseFromIndex(c.Column)
				}
			}
			if err = m.manager.Del(&m.work.Data[3]); err != nil {
				return err
			}
			m.work.Data[3] = numeral.NewRational()
		}
		z := numeral.NewRational()
		if err = m.manager.Copy(alpha, &z); err != nil {
			return err
		}
		m.WorkVector.EraseFromIndex(j)
	}
	if len(m.Rows) == 0 {
		return ErrBounds
	}
	lastRow := m.RowCount() - 1
	for _, j := range m.WorkVector.Index {
		if uint64(j) >= uint64(len(m.WorkVector.Data)) {
			return ErrBounds
		}
		if err = m.fillSet(lastRow, j, &m.WorkVector.Data[j]); err != nil {
			return err
		}
	}
	if len(m.Columns) == 0 {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	one := numeral.NewRational()
	one.Num = numeral.NewInt(1)
	return m.fillSet(lastRow, m.ColumnCount()-1, &one)
}
func (m *RationalStaticMatrix) fillSet(row, column uint32, value *numeral.Rational) (err error) {
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	defer func() { err = m.calls.after(m.manager, b, err) }()
	return m.set(row, column, value, 4, 5)
}
