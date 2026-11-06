package lp

// Source: src/util/lp/static_matrix.h/.hpp, retained exact extraction/products.

import "github.com/glycerine/g3/pkg/util/numeral"

// CopyColumnToIndexedVector leaves existing entries intact and appends the
// nonzero source indices in literal column order. The destination owns its
// own stopped numeric calls; the matrix must not adopt those receipts.
func (m *RationalStaticMatrix) CopyColumnToIndexedVector(column uint32, v *IndexedRationalVector) error {
	if err := m.ready(); err != nil {
		return err
	}
	if v.manager != m.manager {
		return ErrExactVectorOwner
	}
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	if uint64(column) >= uint64(len(m.Columns)) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	for _, c := range m.Columns[column] {
		value, err := m.ColumnValue(c)
		if err != nil {
			return err
		}
		zero, err := m.manager.IsZero(value)
		if err != nil {
			return err
		}
		if !zero {
			if err = v.SetValue(value, c.Row); err != nil {
				return err
			}
		}
	}
	return nil
}

// CopyColumnToVector uses the vector's dense owning storage only. Resize
// preserves retained slots, initializes the new tail and deletes truncated
// values. Zero matrix terms do not overwrite old slots. Index is untouched.
func (m *RationalStaticMatrix) CopyColumnToVector(column uint32, v *IndexedRationalVector) error {
	if err := m.ready(); err != nil {
		return err
	}
	if v.manager != m.manager {
		return ErrExactVectorOwner
	}
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	if err := v.resizeData(m.RowCount()); err != nil {
		return err
	}
	if uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	for _, c := range m.Columns[column] {
		value, err := m.ColumnValue(c)
		if err != nil {
			return err
		}
		zero, err := m.manager.IsZero(value)
		if err != nil {
			return err
		}
		if !zero {
			if uint64(c.Row) >= uint64(len(v.Data)) {
				return ErrBounds
			}
			if err = v.copySlot(&v.Data[c.Row], value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *RationalStaticMatrix) AddColumnToVector(alpha *numeral.Rational, column uint32, v []numeral.Rational) (err error) {
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	product, tmp := &m.work.Data[0], &m.work.Data[1]
	for _, c := range m.Columns[column] {
		if uint64(c.Row) >= uint64(len(v)) {
			return ErrBounds
		}
		value, e := m.ColumnValue(c)
		if e != nil {
			return e
		}
		if err = etaRationalProduct(m.manager, tmp, product, alpha, value); err != nil {
			return err
		}
		if err = m.manager.Add(&v[c.Row], product, &v[c.Row]); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
	}
	return nil
}

// DotColumn and DotRow retain a local accumulator, then copy the completed
// result into initialized caller-owned out. A stopped sum stays with Close;
// out is not used as the accumulator and may alias an input coefficient.
func (m *RationalStaticMatrix) DotColumn(v []numeral.Rational, column uint32, out *numeral.Rational) (err error) {
	b, err := m.beginWork(3)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(column) >= uint64(len(m.Columns)) {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	acc, product, tmp := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
	for _, c := range m.Columns[column] {
		if uint64(c.Row) >= uint64(len(v)) {
			return ErrBounds
		}
		value, e := m.ColumnValue(c)
		if e != nil {
			return e
		}
		if err = etaRationalProduct(m.manager, tmp, product, &v[c.Row], value); err != nil {
			return err
		}
		if err = m.manager.Add(acc, product, acc); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
	}
	return m.manager.Copy(out, acc)
}
func (m *RationalStaticMatrix) DotRow(row uint32, v []numeral.Rational, out *numeral.Rational) (err error) {
	b, err := m.beginWork(3)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(row) >= uint64(len(m.Rows)) {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	acc, product, tmp := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
	for i := range m.Rows[row] {
		c := &m.Rows[row][i]
		if uint64(c.Column) >= uint64(len(v)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, tmp, product, &v[c.Column], &c.Value); err != nil {
			return err
		}
		if err = m.manager.Add(acc, product, acc); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
	}
	return m.manager.Copy(out, acc)
}

// DotPairRow is the live numeric_pair<mpq> row instantiation. Both member and
// free source products form scalar-left rational products, followed by pair
// field copies. Coordinate evaluation uses the existing Go X-before-Y profile.
func (m *RationalStaticMatrix) DotPairRow(row uint32, v []RationalPair, out *RationalPair) (err error) {
	b, err := m.beginWork(7)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(row) >= uint64(len(m.Rows)) {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	w := m.work.Data
	accX, accY, paramX, paramY, prodX, prodY, tmp := &w[0], &w[1], &w[2], &w[3], &w[4], &w[5], &w[6]
	for i := range m.Rows[row] {
		c := &m.Rows[row][i]
		if uint64(c.Column) >= uint64(len(v)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, tmp, paramX, &c.Value, &v[c.Column].X); err != nil {
			return err
		}
		if err = etaRationalProduct(m.manager, tmp, paramY, &c.Value, &v[c.Column].Y); err != nil {
			return err
		}
		if err = m.manager.Copy(prodX, paramX); err != nil {
			return err
		}
		if err = m.manager.Copy(prodY, paramY); err != nil {
			return err
		}
		if err = m.manager.Del(paramY); err != nil {
			return err
		}
		*paramY = numeral.NewRational()
		if err = m.manager.Del(paramX); err != nil {
			return err
		}
		*paramX = numeral.NewRational()
		if err = m.manager.Add(accX, prodX, accX); err != nil {
			return err
		}
		if err = m.manager.Add(accY, prodY, accY); err != nil {
			return err
		}
		if err = m.manager.Del(prodY); err != nil {
			return err
		}
		*prodY = numeral.NewRational()
		if err = m.manager.Del(prodX); err != nil {
			return err
		}
		*prodX = numeral.NewRational()
	}
	if err = m.manager.Copy(&out.X, accX); err != nil {
		return err
	}
	return m.manager.Copy(&out.Y, accY)
}

func (m *RationalStaticMatrix) MaxAbsRow(row uint32, out *numeral.Rational) error {
	return m.absRow(row, false, out)
}
func (m *RationalStaticMatrix) MinAbsRow(row uint32, out *numeral.Rational) error {
	return m.absRow(row, true, out)
}
func (m *RationalStaticMatrix) MaxAbsColumn(column uint32, out *numeral.Rational) error {
	return m.absColumn(column, false, out)
}
func (m *RationalStaticMatrix) MinAbsColumn(column uint32, out *numeral.Rational) error {
	return m.absColumn(column, true, out)
}
func (m *RationalStaticMatrix) absRow(row uint32, minimum bool, out *numeral.Rational) (err error) {
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(row) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	first := minimum
	for i := range m.Rows[row] {
		if err = m.selectAbs(&m.Rows[row][i].Value, minimum, &first); err != nil {
			return err
		}
	}
	return m.manager.Copy(out, &m.work.Data[0])
}
func (m *RationalStaticMatrix) absColumn(column uint32, minimum bool, out *numeral.Rational) (err error) {
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	first := minimum
	for _, c := range m.Columns[column] {
		value, e := m.ColumnValue(c)
		if e != nil {
			return e
		}
		if err = m.selectAbs(value, minimum, &first); err != nil {
			return err
		}
	}
	return m.manager.Copy(out, &m.work.Data[0])
}
func (m *RationalStaticMatrix) selectAbs(value *numeral.Rational, minimum bool, first *bool) error {
	acc, a := &m.work.Data[0], &m.work.Data[1]
	if err := m.manager.Copy(a, value); err != nil {
		return err
	}
	if err := m.manager.Abs(a); err != nil {
		return err
	}
	choose := *first
	if !choose {
		var c int
		var err error
		if minimum {
			c, err = m.manager.Compare(a, acc)
		} else {
			c, err = m.manager.Compare(acc, a)
		}
		if err != nil {
			return err
		}
		choose = c < 0
	}
	if choose {
		if err := m.manager.Copy(acc, a); err != nil {
			return err
		}
	}
	*first = false
	if err := m.manager.Del(a); err != nil {
		return err
	}
	*a = numeral.NewRational()
	return nil
}
