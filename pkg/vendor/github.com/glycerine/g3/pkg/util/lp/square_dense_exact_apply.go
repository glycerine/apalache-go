package lp

// Source: src/util/lp/square_dense_submatrix.hpp:175-197,211-299,325-351;
// square_dense_submatrix.h:182-215. Retained mpq dense/sparse applications.

import "github.com/glycerine/g3/pkg/util/numeral"

// Last four work cells are the row/column sum, product, expression temporary
// and return value. The preceding cells own the full temporary output vector.
func (m *RationalSquareDenseSubmatrix) rowProduct(i uint32, w []numeral.Rational, out *numeral.Rational) error {
	if i < m.IndexStart {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	base := len(m.work.Data) - 4
	sum, product, tmp := &m.work.Data[base], &m.work.Data[base+1], &m.work.Data[base+2]
	offset := (i - m.IndexStart) * m.Dim
	for j := uint32(0); j < m.Dim; j++ {
		col, err := m.AdjustColumnInverse(m.IndexStart + j)
		if err != nil {
			return err
		}
		if uint64(offset+j) >= uint64(len(m.Data)) || uint64(col) >= uint64(len(w)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, tmp, product, &m.Data[offset+j], &w[col]); err != nil {
			return err
		}
		if err = m.manager.Add(sum, product, sum); err != nil {
			return err
		}
		if err = m.deleteSlot(base + 1); err != nil {
			return err
		}
	}
	if err := m.manager.Copy(out, sum); err != nil {
		return err
	}
	return m.deleteSlot(base)
}
func (m *RationalSquareDenseSubmatrix) columnProduct(j uint32, w []numeral.Rational, out *numeral.Rational) error {
	if j < m.IndexStart {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	base := len(m.work.Data) - 4
	sum, product, tmp := &m.work.Data[base], &m.work.Data[base+1], &m.work.Data[base+2]
	offset := j - m.IndexStart
	for i := uint32(0); i < m.Dim; i++ {
		row, err := m.AdjustRowInverse(m.IndexStart + i)
		if err != nil {
			return err
		}
		if uint64(offset) >= uint64(len(m.Data)) || uint64(row) >= uint64(len(w)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, tmp, product, &m.Data[offset], &w[row]); err != nil {
			return err
		}
		if err = m.manager.Add(sum, product, sum); err != nil {
			return err
		}
		if err = m.deleteSlot(base + 1); err != nil {
			return err
		}
		offset += m.Dim
	}
	if err := m.manager.Copy(out, sum); err != nil {
		return err
	}
	return m.deleteSlot(base)
}
func (m *RationalSquareDenseSubmatrix) ApplyLeft(w []numeral.Rational) (err error) {
	n := m.Parent.Dimension()
	b, err := m.beginWork(n + 4)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	m.vectorPrefix = n
	t := m.work.Data[:n]
	for i := uint32(0); i < m.IndexStart; i++ {
		row, e := m.AdjustRowInverse(i)
		if e != nil {
			return e
		}
		col, e := m.AdjustColumnInverse(i)
		if e != nil {
			return e
		}
		if uint64(row) >= uint64(len(t)) || uint64(col) >= uint64(len(w)) {
			return ErrBounds
		}
		if err = m.manager.Copy(&t[row], &w[col]); err != nil {
			return err
		}
	}
	result := &m.work.Data[n+3]
	for i := m.IndexStart; i < n; i++ {
		row, e := m.AdjustRowInverse(i)
		if e != nil {
			return e
		}
		if err = m.rowProduct(i, w, result); err != nil {
			return err
		}
		if uint64(row) >= uint64(len(t)) {
			return ErrBounds
		}
		if err = m.manager.Copy(&t[row], result); err != nil {
			return err
		}
		if err = m.deleteSlot(int(n + 3)); err != nil {
			return err
		}
	}
	for i := uint32(0); i < n; i++ {
		if uint64(i) >= uint64(len(w)) {
			return ErrBounds
		}
		if err = m.manager.Copy(&w[i], &t[i]); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalSquareDenseSubmatrix) ApplyRight(w []numeral.Rational) (err error) {
	// Native slices retain the caller's length; the source temporary also has
	// w.size(), and its assignment writes initialized zeros to trailing slots.
	b, err := m.beginWork(uint32(len(w)) + 4)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	m.vectorPrefix = uint32(len(w))
	t := m.work.Data[:len(w)]
	end := m.IndexStart + m.Dim
	for j := uint32(0); j < m.IndexStart; j++ {
		col, e := m.AdjustColumnInverse(j)
		if e != nil {
			return e
		}
		row, e := m.AdjustRowInverse(j)
		if e != nil {
			return e
		}
		if uint64(col) >= uint64(len(t)) || uint64(row) >= uint64(len(w)) {
			return ErrBounds
		}
		if err = m.manager.Copy(&t[col], &w[row]); err != nil {
			return err
		}
	}
	for j := end; j < m.Parent.Dimension(); j++ {
		col, e := m.AdjustColumnInverse(j)
		if e != nil {
			return e
		}
		row, e := m.AdjustRowInverse(j)
		if e != nil {
			return e
		}
		if uint64(col) >= uint64(len(t)) || uint64(row) >= uint64(len(w)) {
			return ErrBounds
		}
		if err = m.manager.Copy(&t[col], &w[row]); err != nil {
			return err
		}
	}
	result := &m.work.Data[len(w)+3]
	for j := m.IndexStart; j < end; j++ {
		col, e := m.AdjustColumnInverse(j)
		if e != nil {
			return e
		}
		if err = m.columnProduct(j, w, result); err != nil {
			return err
		}
		if uint64(col) >= uint64(len(t)) {
			return ErrBounds
		}
		if err = m.manager.Copy(&t[col], result); err != nil {
			return err
		}
		if err = m.deleteSlot(len(w) + 3); err != nil {
			return err
		}
	}
	for i := range w {
		if err = m.manager.Copy(&w[i], &t[i]); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalSquareDenseSubmatrix) indexedInput(w *IndexedRationalVector) error {
	if err := m.ready(); err != nil {
		return err
	}
	if m.manager != w.manager {
		return ErrExactVectorOwner
	}
	if w.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}
func (m *RationalSquareDenseSubmatrix) ApplyLeftToT(w *IndexedRationalVector, _ *Settings) (err error) {
	if err = m.indexedInput(w); err != nil {
		return err
	}
	n := m.Parent.Dimension()
	b, err := m.beginWork(n + 2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	m.vectorPrefix = n
	t, product, tmp := m.work.Data[:n], &m.work.Data[n], &m.work.Data[n+1]
	for _, k := range w.Index {
		j, e := m.AdjustColumn(k)
		if e != nil {
			return e
		}
		if uint64(k) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if j < m.IndexStart || j >= m.IndexStart+m.Dim {
			row, e := m.AdjustRowInverse(j)
			if e != nil {
				return e
			}
			if uint64(row) >= uint64(len(t)) {
				return ErrBounds
			}
			if err = m.manager.Copy(&t[row], &w.Data[k]); err != nil {
				return err
			}
		} else {
			for i := uint32(0); i < m.Dim; i++ {
				row, e := m.AdjustRowInverse(m.IndexStart + i)
				if e != nil {
					return e
				}
				offset := i*m.Dim + j - m.IndexStart
				if uint64(row) >= uint64(len(t)) || uint64(offset) >= uint64(len(m.Data)) {
					return ErrBounds
				}
				if err = etaRationalProduct(m.manager, tmp, product, &m.Data[offset], &w.Data[k]); err != nil {
					return err
				}
				if err = m.manager.Add(&t[row], product, &t[row]); err != nil {
					return err
				}
				if err = m.deleteSlot(int(n)); err != nil {
					return err
				}
			}
		}
	}
	w.Index = w.Index[:0]
	for i := uint32(0); i < n; i++ {
		zero, e := m.manager.IsZero(&t[i])
		if e != nil {
			return e
		}
		if !zero {
			w.Index = append(w.Index, i)
		}
		if uint64(i) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if err = m.manager.Copy(&w.Data[i], &t[i]); err != nil {
			return err
		}
	}
	return nil
}

// The product belongs to this block; Add belongs to the persistent indexed
// workspace. Capture the two actual invocations separately.
func (m *RationalSquareDenseSubmatrix) indexedRightProduct(coefficient, value *numeral.Rational) (err error) {
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	defer func() { err = m.calls.after(m.manager, b, err) }()
	return etaRationalProduct(m.manager, &m.work.Data[1], &m.work.Data[0], coefficient, value)
}
func (m *RationalSquareDenseSubmatrix) ApplyRightIndexed(w *IndexedRationalVector) (err error) {
	if err = m.indexedInput(w); err != nil {
		return err
	}
	// No broad capture across delegated vector operations: each vector owns
	// only its new numerical invocation, including the caller's final Assign.
	m.work = NewIndexedRationalVector(m.manager, 2)
	defer m.finishParentUpdate(&err)
	if err = m.WorkVector.Resize(w.DataSize()); err != nil {
		return err
	}
	if err = m.WorkVector.Clear(); err != nil {
		return err
	}
	end := m.IndexStart + m.Dim
	for _, k := range w.Index {
		j, e := m.AdjustRow(k)
		if e != nil {
			return e
		}
		if uint64(k) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if j < m.IndexStart || j >= end {
			col, e := m.AdjustColumnInverse(j)
			if e != nil {
				return e
			}
			if err = m.WorkVector.SetValue(&w.Data[k], col); err != nil {
				return err
			}
		} else {
			offset := (j - m.IndexStart) * m.Dim
			for col := m.IndexStart; col < end; col++ {
				adj, e := m.AdjustColumnInverse(col)
				if e != nil {
					return e
				}
				if uint64(offset) >= uint64(len(m.Data)) {
					return ErrBounds
				}
				if err = m.indexedRightProduct(&m.Data[offset], &w.Data[k]); err != nil {
					return err
				}
				if err = m.WorkVector.Add(adj, &m.work.Data[0]); err != nil {
					return err
				}
				if err = m.deleteSlot(0); err != nil {
					return err
				}
				offset++
			}
		}
	}
	if err = m.WorkVector.CleanUp(); err != nil {
		return err
	}
	return w.Assign(m.WorkVector)
}
func (m *RationalSquareDenseSubmatrix) Conjugate(p *Permutation) error {
	if err := m.ready(); err != nil {
		return err
	}
	if err := m.RowPermutation.MultiplyLeft(p); err != nil {
		return err
	}
	return m.ColumnPermutation.MultiplyReverseRight(p)
}

var _ RationalTailMatrix = (*RationalSquareDenseSubmatrix)(nil)
