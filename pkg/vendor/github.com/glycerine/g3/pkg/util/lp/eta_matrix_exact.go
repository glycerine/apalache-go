package lp

// Source: src/util/lp/eta_matrix.h/.hpp, retained mpq/mpq and mpq/pair instances.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalEtaMatrix owns exact coefficients and lexical arithmetic values in a
// borrowed manager. Caller vectors retain their completed and partial updates.
// A stopped application is closed before further mutation. Do not shallow-copy.
type RationalEtaMatrix struct {
	column   uint32
	Column   *SparseRationalVector
	Diagonal numeral.Rational
	manager  *numeral.RationalManager
	calls    exactEtaCalls
	work     *IndexedRationalVector
}

func NewRationalEtaMatrix(m *numeral.RationalManager, column, _ uint32) *RationalEtaMatrix {
	return &RationalEtaMatrix{column: column, manager: m, Column: NewSparseRationalVector(m), Diagonal: numeral.NewRational()}
}
func (m *RationalEtaMatrix) IsDense() bool       { return false }
func (m *RationalEtaMatrix) ColumnIndex() uint32 { return m.column }
func (m *RationalEtaMatrix) ready() error {
	if m.work != nil || m.calls.pending() {
		return ErrExactVectorPending
	}
	return m.Column.ready()
}
func (m *RationalEtaMatrix) beginWork(size uint32) (exactEtaBefore, error) {
	if err := m.ready(); err != nil {
		return exactEtaBefore{}, err
	}
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return b, err
	}
	m.work = NewIndexedRationalVector(m.manager, size)
	return b, nil
}
func (m *RationalEtaMatrix) finishWork(b exactEtaBefore, err *error) {
	*err = m.calls.after(m.manager, b, *err)
	if *err == nil {
		*err = m.work.Close()
		if *err == nil {
			m.work = nil
		}
	}
}

func (m *RationalEtaMatrix) SetDiagonal(value *numeral.Rational) (ok bool, err error) {
	b, err := m.beginWork(0)
	if err != nil {
		return false, err
	}
	defer m.finishWork(b, &err)
	if err = m.manager.Copy(&m.Diagonal, value); err != nil {
		return false, err
	}
	zero, err := m.manager.IsZero(value)
	return !zero, err
}
func (m *RationalEtaMatrix) Append(row uint32, value *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	// The outer eta value parameter is copied before its assertion, followed
	// by sparse_vector's own value parameter and stored term construction.
	m.work = NewIndexedRationalVector(m.manager, 1)
	if err = m.work.copySlot(&m.work.Data[0], value); err != nil {
		return err
	}
	if row == m.column {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if err = m.Column.Append(row, &m.work.Data[0]); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}
func (m *RationalEtaMatrix) DivideByDiagonal() error {
	if err := m.ready(); err != nil {
		return err
	}
	return m.Column.Divide(&m.Diagonal)
}

func (m *RationalEtaMatrix) ApplyLeft(w []numeral.Rational) (err error) {
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(m.column) >= uint64(len(w)) {
		return ErrBounds
	}
	product := &m.work.Data[0]
	for i := range m.Column.Data {
		e := &m.Column.Data[i]
		if uint64(e.Index) >= uint64(len(w)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, &m.work.Data[1], product, &w[m.column], &e.Value); err != nil {
			return err
		}
		if err = m.manager.Add(&w[e.Index], product, &w[e.Index]); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
	}
	return m.manager.Div(&w[m.column], &m.Diagonal, &w[m.column])
}
func (m *RationalEtaMatrix) ApplyRight(w []numeral.Rational) (err error) {
	b, err := m.beginWork(3)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(m.column) >= uint64(len(w)) {
		return ErrBounds
	}
	value, product := &m.work.Data[0], &m.work.Data[1]
	if err = etaRationalQuotient(m.manager, &m.work.Data[2], value, &w[m.column], &m.Diagonal); err != nil {
		return err
	}
	for i := range m.Column.Data {
		e := &m.Column.Data[i]
		if uint64(e.Index) >= uint64(len(w)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, &m.work.Data[2], product, &w[e.Index], &e.Value); err != nil {
			return err
		}
		if err = m.manager.Add(value, product, value); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
	}
	return m.manager.Copy(&w[m.column], value)
}
func (m *RationalEtaMatrix) ApplyLeftToT(w *IndexedRationalVector, _ *Settings) (err error) {
	if m.manager != w.manager {
		return ErrExactVectorOwner
	}
	if w.calls.pending() {
		return ErrExactVectorPending
	}
	b, err := m.beginWork(4)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(m.column) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	anchor, product, value := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
	if err = m.manager.Copy(anchor, &w.Data[m.column]); err != nil {
		return err
	}
	zero, err := m.manager.IsZero(anchor)
	if err != nil || zero {
		return err
	}
	if err = m.manager.Div(&w.Data[m.column], &m.Diagonal, &w.Data[m.column]); err != nil {
		return err
	}
	zero, err = m.manager.IsZero(&w.Data[m.column])
	if err != nil {
		return err
	}
	if zero {
		z := numeral.NewRational()
		if err = m.manager.Copy(&w.Data[m.column], &z); err != nil {
			return err
		}
		w.EraseFromIndex(m.column)
	}
	for i := range m.Column.Data {
		e := &m.Column.Data[i]
		if uint64(e.Index) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		dst := &w.Data[e.Index]
		wasZero, e2 := m.manager.IsZero(dst)
		if e2 != nil {
			return e2
		}
		if err = etaRationalProduct(m.manager, &m.work.Data[3], product, anchor, &e.Value); err != nil {
			return err
		}
		if wasZero {
			err = m.manager.Copy(dst, product)
		} else {
			err = m.manager.Add(dst, product, dst)
		}
		if err != nil {
			return err
		}
		// Source L v copies the updated destination before its drop test.
		if err = m.manager.Copy(value, dst); err != nil {
			return err
		}
		// The RHS product dies at the end of the L v initialization,
		// before the drop test; v itself lives through the branch below.
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
		zero, err = m.manager.IsZero(value)
		if err != nil {
			return err
		}
		if zero {
			z := numeral.NewRational()
			if err = m.manager.Copy(dst, &z); err != nil {
				return err
			}
			if !wasZero {
				w.EraseFromIndex(e.Index)
			}
		} else if wasZero {
			w.Index = append(w.Index, e.Index)
		}
		if err = m.manager.Del(value); err != nil {
			return err
		}
		*value = numeral.NewRational()
	}
	return nil
}
func (m *RationalEtaMatrix) ApplyRightIndexed(w *IndexedRationalVector) (err error) {
	if m.manager != w.manager {
		return ErrExactVectorOwner
	}
	if w.calls.pending() {
		return ErrExactVectorPending
	}
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if len(w.Index) == 0 {
		return nil
	}
	if uint64(m.column) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	value, product := &w.Data[m.column], &m.work.Data[0]
	if err = m.manager.Div(value, &m.Diagonal, value); err != nil {
		return err
	}
	wasZero, err := m.manager.IsZero(value)
	if err != nil {
		return err
	}
	for i := range m.Column.Data {
		e := &m.Column.Data[i]
		if uint64(e.Index) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, &m.work.Data[1], product, &w.Data[e.Index], &e.Value); err != nil {
			return err
		}
		if err = m.manager.Add(value, product, value); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
	}
	zero, err := m.manager.IsZero(value)
	if err != nil {
		return err
	}
	if !zero && wasZero {
		w.Index = append(w.Index, m.column)
	} else if zero && !wasZero {
		w.EraseFromIndex(m.column)
	}
	return nil
}
func (m *RationalEtaMatrix) Conjugate(p *Permutation) error {
	if err := m.ready(); err != nil {
		return err
	}
	column, err := p.Reverse(m.column)
	if err != nil {
		return err
	}
	m.column = column
	for i := range m.Column.Data {
		row, e := p.Reverse(m.Column.Data[i].Index)
		if e != nil {
			return e
		}
		m.Column.Data[i].Index = row
	}
	return nil
}
func (m *RationalEtaMatrix) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if err := m.Column.closeCall(); err != nil {
		return err
	}
	if m.work != nil {
		if err := m.work.Close(); err != nil {
			return err
		}
		m.work = nil
	}
	if err := m.manager.Del(&m.Diagonal); err != nil {
		return err
	}
	m.Diagonal = numeral.NewRational()
	// C++ members die in reverse declaration order: diagonal, then column.
	if err := m.Column.Close(); err != nil {
		return err
	}
	return m.calls.cleanupRoots(m.manager)
}

// PairEtaMatrix shares the rational coefficient and right/sparse algorithms;
// its dense left application acts on the two exact coordinates of X.
type PairEtaMatrix struct {
	*RationalEtaMatrix
	pairWork *IndexedPairVector
}

func NewPairEtaMatrix(m *numeral.RationalManager, column, length uint32) *PairEtaMatrix {
	return &PairEtaMatrix{RationalEtaMatrix: NewRationalEtaMatrix(m, column, length)}
}
func (m *PairEtaMatrix) ApplyLeft(w []RationalPair) (err error) {
	b, err := m.beginWork(3)
	if err != nil {
		return err
	}
	m.pairWork = NewIndexedPairVector(m.manager, 1)
	defer func() {
		err = m.calls.after(m.manager, b, err)
		if err == nil {
			err = m.pairWork.Close()
			if err == nil {
				m.pairWork = nil
				err = m.work.Close()
				if err == nil {
					m.work = nil
				}
			}
		}
	}()
	if uint64(m.column) >= uint64(len(w)) {
		return ErrBounds
	}
	product := &m.pairWork.Data[0]
	for i := range m.Column.Data {
		e := &m.Column.Data[i]
		if uint64(e.Index) >= uint64(len(w)) {
			return ErrBounds
		}
		// Source numeric_pair::operator* constructs its coordinates from
		// coefficient * coordinate, then copies both expression values.
		x, y, tmp := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
		if err = etaRationalProduct(m.manager, tmp, x, &e.Value, &w[m.column].X); err != nil {
			return err
		}
		if err = etaRationalProduct(m.manager, tmp, y, &e.Value, &w[m.column].Y); err != nil {
			return err
		}
		if err = m.manager.Copy(&product.X, x); err != nil {
			return err
		}
		if err = m.manager.Copy(&product.Y, y); err != nil {
			return err
		}
		if err = m.manager.Del(y); err != nil {
			return err
		}
		*y = numeral.NewRational()
		if err = m.manager.Del(x); err != nil {
			return err
		}
		*x = numeral.NewRational()
		if err = w[e.Index].Add(m.manager, &w[e.Index], product); err != nil {
			return err
		}
		if err = product.Del(m.manager); err != nil {
			return err
		}
	}
	return w[m.column].Divide(m.manager, &w[m.column], &m.Diagonal)
}
func (m *PairEtaMatrix) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if m.pairWork != nil {
		if err := m.pairWork.Close(); err != nil {
			return err
		}
		m.pairWork = nil
	}
	return m.RationalEtaMatrix.Close()
}
