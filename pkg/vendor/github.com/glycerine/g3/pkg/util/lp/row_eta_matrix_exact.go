package lp

// Source: src/util/lp/row_eta_matrix.h/.hpp, retained precise instances.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalRowEtaMatrix owns the literal terms of an identity-plus-one-row
// transformation in a borrowed manager. The unused source row_start argument
// is accepted without storage. Stopped lexical values remain owned until Close.
type RationalRowEtaMatrix struct {
	row     uint32
	Row     *SparseRationalVector
	manager *numeral.RationalManager
	calls   exactEtaCalls
	work    *IndexedRationalVector
}

func NewRationalRowEtaMatrix(m *numeral.RationalManager, _ uint32, row, _ uint32) *RationalRowEtaMatrix {
	return &RationalRowEtaMatrix{row: row, manager: m, Row: NewSparseRationalVector(m)}
}
func (m *RationalRowEtaMatrix) IsDense() bool    { return false }
func (m *RationalRowEtaMatrix) RowIndex() uint32 { return m.row }
func (m *RationalRowEtaMatrix) ready() error {
	if m.work != nil || m.calls.pending() {
		return ErrExactVectorPending
	}
	return m.Row.ready()
}
func (m *RationalRowEtaMatrix) beginWork(size uint32) (exactEtaBefore, error) {
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
func (m *RationalRowEtaMatrix) finishWork(b exactEtaBefore, err *error) {
	*err = m.calls.after(m.manager, b, *err)
	if *err == nil {
		*err = m.work.Close()
		if *err == nil {
			m.work = nil
		}
	}
}
func (m *RationalRowEtaMatrix) Append(index uint32, value *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	m.work = NewIndexedRationalVector(m.manager, 1)
	if err = m.work.copySlot(&m.work.Data[0], value); err != nil {
		return err
	}
	if index == m.row {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if err = m.Row.Append(index, &m.work.Data[0]); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

func (m *RationalRowEtaMatrix) ApplyLeft(w []numeral.Rational) (err error) {
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(m.row) >= uint64(len(w)) {
		return ErrBounds
	}
	product, tmp := &m.work.Data[0], &m.work.Data[1]
	for i := range m.Row.Data {
		e := &m.Row.Data[i]
		if uint64(e.Index) >= uint64(len(w)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, tmp, product, &w[e.Index], &e.Value); err != nil {
			return err
		}
		if err = m.manager.Add(&w[m.row], product, &w[m.row]); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
	}
	return nil
}
func (m *RationalRowEtaMatrix) ApplyLeftToT(w *IndexedRationalVector, _ *Settings) (err error) {
	if m.manager != w.manager {
		return ErrExactVectorOwner
	}
	if w.calls.pending() {
		return ErrExactVectorPending
	}
	b, err := m.beginWork(3)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(m.row) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	value, product, tmp := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
	if err = m.manager.Copy(value, &w.Data[m.row]); err != nil {
		return err
	}
	wasZero, err := m.manager.IsZero(value)
	if err != nil {
		return err
	}
	for i := range m.Row.Data {
		e := &m.Row.Data[i]
		if uint64(e.Index) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, tmp, product, &w.Data[e.Index], &e.Value); err != nil {
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
	if !zero {
		// Source index publication precedes assignment of the local result.
		if wasZero {
			w.Index = append(w.Index, m.row)
		}
		return m.manager.Copy(&w.Data[m.row], value)
	}
	if !wasZero {
		z := numeral.NewRational()
		if err = m.manager.Copy(&w.Data[m.row], &z); err != nil {
			return err
		}
		// The source erase(find(...)) has no end guard. Preserve the zero
		// assignment before rejecting an absent erase position.
		found := false
		for _, i := range w.Index {
			if i == m.row {
				found = true
				break
			}
		}
		if !found {
			return ErrBounds
		}
		w.EraseFromIndex(m.row)
	}
	return nil
}
func (m *RationalRowEtaMatrix) ApplyRight(w []numeral.Rational) (err error) {
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(m.row) >= uint64(len(w)) {
		return ErrBounds
	}
	zero, err := m.manager.IsZero(&w[m.row])
	if err != nil || zero {
		return err
	}
	product, tmp := &m.work.Data[0], &m.work.Data[1]
	for i := range m.Row.Data {
		e := &m.Row.Data[i]
		if uint64(e.Index) >= uint64(len(w)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, tmp, product, &w[m.row], &e.Value); err != nil {
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
	return nil
}
func (m *RationalRowEtaMatrix) ApplyRightIndexed(w *IndexedRationalVector) (err error) {
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
	if uint64(m.row) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	zero, err := m.manager.IsZero(&w.Data[m.row])
	if err != nil || zero {
		return err
	}
	product, tmp := &m.work.Data[0], &m.work.Data[1]
	for i := range m.Row.Data {
		e := &m.Row.Data[i]
		if uint64(e.Index) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		value := &w.Data[e.Index]
		wasZero, e2 := m.manager.IsZero(value)
		if e2 != nil {
			return e2
		}
		if err = etaRationalProduct(m.manager, tmp, product, &w.Data[m.row], &e.Value); err != nil {
			return err
		}
		if err = m.manager.Add(value, product, value); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
		zero, err = m.manager.IsZero(value)
		if err != nil {
			return err
		}
		if wasZero && !zero {
			w.Index = append(w.Index, e.Index)
		} else if !wasZero && zero {
			w.EraseFromIndex(e.Index)
		}
	}
	return nil
}
func (m *RationalRowEtaMatrix) Conjugate(p *Permutation) error {
	if err := m.ready(); err != nil {
		return err
	}
	row, err := p.Reverse(m.row)
	if err != nil {
		return err
	}
	m.row = row
	columns := make([]uint32, len(m.Row.Data))
	for i, e := range m.Row.Data {
		columns[i] = e.Index
	}
	for i := len(columns); i > 0; {
		i--
		index, e := p.Reverse(columns[i])
		if e != nil {
			return e
		}
		m.Row.Data[i].Index = index
	}
	return nil
}
func (m *RationalRowEtaMatrix) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if err := m.Row.closeCall(); err != nil {
		return err
	}
	if m.work != nil {
		if err := m.work.Close(); err != nil {
			return err
		}
		m.work = nil
	}
	// Call locals die before the stored row member.
	if err := m.Row.Close(); err != nil {
		return err
	}
	return m.calls.cleanupRoots(m.manager)
}

// PairRowEtaMatrix shares actual rational coefficients, right applications and
// sparse-to-T applications, with a concrete pair dense-left implementation.
type PairRowEtaMatrix struct {
	*RationalRowEtaMatrix
	pairWork *IndexedPairVector
}

func NewPairRowEtaMatrix(m *numeral.RationalManager, start, row, length uint32) *PairRowEtaMatrix {
	return &PairRowEtaMatrix{RationalRowEtaMatrix: NewRationalRowEtaMatrix(m, start, row, length)}
}
func (m *PairRowEtaMatrix) ApplyLeft(w []RationalPair) (err error) {
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
	if uint64(m.row) >= uint64(len(w)) {
		return ErrBounds
	}
	product := &m.pairWork.Data[0]
	for i := range m.Row.Data {
		e := &m.Row.Data[i]
		if uint64(e.Index) >= uint64(len(w)) {
			return ErrBounds
		}
		x, y, tmp := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
		if err = etaRationalProduct(m.manager, tmp, x, &e.Value, &w[e.Index].X); err != nil {
			return err
		}
		if err = etaRationalProduct(m.manager, tmp, y, &e.Value, &w[e.Index].Y); err != nil {
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
		if err = w[m.row].Add(m.manager, &w[m.row], product); err != nil {
			return err
		}
		if err = product.Del(m.manager); err != nil {
			return err
		}
	}
	return nil
}
func (m *PairRowEtaMatrix) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if m.pairWork != nil {
		if err := m.pairWork.Close(); err != nil {
			return err
		}
		m.pairWork = nil
	}
	return m.RationalRowEtaMatrix.Close()
}

var _ RationalTailMatrix = (*RationalRowEtaMatrix)(nil)
var _ PairTailMatrix = (*PairRowEtaMatrix)(nil)
