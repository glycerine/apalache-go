package lp

// Source: src/util/lp/square_dense_submatrix.h/.hpp. Shared mpq coefficients
// for retained mpq/mpq and mpq/numeric_pair LU dense blocks.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalSquareDenseSubmatrix owns each dense coefficient independently of its
// borrowed sparse parent. A failed constructor or operation retains its owning
// prefix and numerical receipts until Close. Do not copy this owner.
type RationalSquareDenseSubmatrix struct {
	IndexStart, Dim                   uint32
	Data                              []numeral.Rational
	Parent                            *RationalSparseMatrix
	RowPermutation, ColumnPermutation *Permutation
	WorkVector                        *IndexedRationalVector
	manager                           *numeral.RationalManager
	calls                             exactEtaCalls
	work                              *IndexedRationalVector
	vectorPrefix                      uint32 // leading output-vector cells, destroyed forward
}

func NewRationalSquareDenseSubmatrix(parent *RationalSparseMatrix, start uint32) (m *RationalSquareDenseSubmatrix, err error) {
	if err = parent.ready(); err != nil {
		return nil, err
	}
	m = &RationalSquareDenseSubmatrix{IndexStart: start, Dim: parent.Dimension() - start, Parent: parent, manager: parent.manager,
		RowPermutation: NewPermutation(parent.Dimension()), ColumnPermutation: NewPermutation(parent.Dimension()), WorkVector: NewIndexedRationalVector(parent.manager, 0)}
	if start > parent.Dimension() {
		return m, ErrBounds
	}
	m.Data = make([]numeral.Rational, m.Dim*m.Dim)
	for i := range m.Data {
		m.Data[i] = numeral.NewRational()
	}
	b, err := m.beginWork(0)
	if err != nil {
		return m, err
	}
	defer m.finishWork(b, &err)
	offset := uint32(0) - start
	for i := start; i < parent.Dimension(); i++ {
		row, e := parent.AdjustRow(i)
		if e != nil {
			return m, e
		}
		if uint64(row) >= uint64(len(parent.Rows)) {
			return m, ErrBounds
		}
		for k := range parent.Rows[row] {
			v := &parent.Rows[row][k]
			j, e := parent.AdjustColumnInverse(v.Index)
			if e != nil {
				return m, e
			}
			if j < start {
				if e = sourceAssertionFailure(); e != nil {
					return m, e
				}
			}
			at := offset + j
			if uint64(at) >= uint64(len(m.Data)) {
				return m, ErrBounds
			}
			if err = m.manager.Copy(&m.Data[at], &v.Value); err != nil {
				return m, err
			}
		}
		offset += m.Dim
	}
	return m, nil
}
func (m *RationalSquareDenseSubmatrix) IsDense() bool { return true }
func (m *RationalSquareDenseSubmatrix) ready() error {
	if m.work != nil || m.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}
func (m *RationalSquareDenseSubmatrix) beginWork(size uint32) (exactEtaBefore, error) {
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
func (m *RationalSquareDenseSubmatrix) finishWork(b exactEtaBefore, err *error) {
	*err = m.calls.after(m.manager, b, *err)
	if *err == nil {
		*err = m.closeWork()
		if *err == nil {
			m.work = nil
		}
	}
}
func (m *RationalSquareDenseSubmatrix) deleteSlot(i int) error {
	if err := m.manager.Del(&m.work.Data[i]); err != nil {
		return err
	}
	m.work.Data[i] = numeral.NewRational()
	return nil
}
func (m *RationalSquareDenseSubmatrix) AdjustColumn(j uint32) (uint32, error) {
	if j >= m.ColumnPermutation.Size() {
		return j, nil
	}
	return m.ColumnPermutation.Reverse(j)
}
func (m *RationalSquareDenseSubmatrix) AdjustColumnInverse(j uint32) (uint32, error) {
	if j >= m.ColumnPermutation.Size() {
		return j, nil
	}
	return m.ColumnPermutation.Value(j)
}
func (m *RationalSquareDenseSubmatrix) AdjustRow(i uint32) (uint32, error) {
	if i >= m.RowPermutation.Size() {
		return i, nil
	}
	return m.RowPermutation.Value(i)
}
func (m *RationalSquareDenseSubmatrix) AdjustRowInverse(i uint32) (uint32, error) {
	if i >= m.RowPermutation.Size() {
		return i, nil
	}
	return m.RowPermutation.Reverse(i)
}
func (m *RationalSquareDenseSubmatrix) cell(i, j uint32) (*numeral.Rational, error) {
	if i < m.IndexStart || i >= m.Parent.Dimension() || j < m.IndexStart {
		if err := sourceAssertionFailure(); err != nil {
			return nil, err
		}
	}
	j, err := m.AdjustColumn(j)
	if err != nil {
		return nil, err
	}
	offset := (i-m.IndexStart)*m.Dim + j - m.IndexStart
	if uint64(offset) >= uint64(len(m.Data)) {
		return nil, ErrBounds
	}
	return &m.Data[offset], nil
}
func (m *RationalSquareDenseSubmatrix) SwapColumns(i, j uint32) error {
	if err := m.ready(); err != nil {
		return err
	}
	if i == j {
		return nil
	}
	return m.ColumnPermutation.TransposeLeft(i, j)
}
func (m *RationalSquareDenseSubmatrix) FindPivotColumn(i uint32) (best int32, err error) {
	best = -1
	b, err := m.beginWork(2)
	if err != nil {
		return best, err
	}
	defer m.finishWork(b, &err)
	if i < m.IndexStart {
		if err = sourceAssertionFailure(); err != nil {
			return best, err
		}
	}
	maximum, candidate := &m.work.Data[0], &m.work.Data[1]
	row := (i - m.IndexStart) * m.Dim
	for k := i; k < m.Parent.Dimension(); k++ {
		col, e := m.AdjustColumn(k)
		if e != nil {
			return best, e
		}
		offset := row + col - m.IndexStart
		if uint64(offset) >= uint64(len(m.Data)) {
			return best, ErrBounds
		}
		if err = m.manager.Copy(candidate, &m.Data[offset]); err != nil {
			return best, err
		}
		if err = m.manager.Abs(candidate); err != nil {
			return best, err
		}
		cmp, e := m.manager.Compare(candidate, maximum)
		if e != nil {
			return best, e
		}
		if cmp > 0 {
			best = int32(k)
			if err = m.manager.Copy(maximum, candidate); err != nil {
				return best, err
			}
		}
		if err = m.deleteSlot(1); err != nil {
			return best, err
		}
	}
	return best, nil
}
func (m *RationalSquareDenseSubmatrix) DivideRowByPivot(i uint32) (err error) {
	b, err := m.beginWork(3)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	pj, err := m.AdjustColumn(i)
	if err != nil {
		return err
	}
	offset := (i - m.IndexStart) * m.Dim
	pk := offset + pj - m.IndexStart
	if uint64(pk) >= uint64(len(m.Data)) {
		return ErrBounds
	}
	pivot := &m.work.Data[0]
	if err = m.manager.Copy(pivot, &m.Data[pk]); err != nil {
		return err
	}
	if sourceDebugEnabled {
		zero, e := m.manager.IsZero(pivot)
		if e != nil {
			return e
		}
		if zero {
			return ErrAssertion
		}
	}
	for k := m.IndexStart; k < m.Parent.Dimension(); k++ {
		if uint64(offset) >= uint64(len(m.Data)) {
			return ErrBounds
		}
		if k == pj {
			one := numeral.NewRational()
			one.Num = numeral.NewInt(1)
			if err = etaRationalQuotient(m.manager, &m.work.Data[1], &m.work.Data[2], &one, pivot); err != nil {
				return err
			}
			if err = m.manager.Copy(&m.Data[offset], &m.work.Data[2]); err != nil {
				return err
			}
			if err = m.deleteSlot(2); err != nil {
				return err
			}
		} else if err = m.manager.Div(&m.Data[offset], pivot, &m.Data[offset]); err != nil {
			return err
		}
		offset++
	}
	return m.deleteSlot(0)
}
func (m *RationalSquareDenseSubmatrix) PivotRowToRow(i, row uint32, _ *Settings) (err error) {
	b, err := m.beginWork(7)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if i >= row {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	pj, err := m.AdjustColumn(i)
	if err != nil {
		return err
	}
	po, ro := (i-m.IndexStart)*m.Dim, (row-m.IndexStart)*m.Dim
	pk, rk := po+pj-m.IndexStart, ro+pj-m.IndexStart
	if uint64(pk) >= uint64(len(m.Data)) {
		return ErrBounds
	}
	pivot, mult := &m.work.Data[0], &m.work.Data[1]
	if err = m.manager.Copy(pivot, &m.Data[pk]); err != nil {
		return err
	}
	if uint64(rk) >= uint64(len(m.Data)) {
		return ErrBounds
	}
	if err = m.manager.Copy(mult, &m.Data[rk]); err != nil {
		return err
	}
	if sourceDebugEnabled {
		zero, e := m.manager.IsZero(pivot)
		if e != nil {
			return e
		}
		if zero {
			return ErrAssertion
		}
	}
	// Retain unary minus through multiplication and target assignment.
	if err = m.manager.Copy(&m.work.Data[2], mult); err != nil {
		return err
	}
	if err = m.manager.Neg(&m.work.Data[2]); err != nil {
		return err
	}
	if err = etaRationalProduct(m.manager, &m.work.Data[3], &m.work.Data[4], &m.work.Data[2], pivot); err != nil {
		return err
	}
	if err = m.manager.Copy(&m.Data[rk], &m.work.Data[4]); err != nil {
		return err
	}
	if err = m.deleteSlot(4); err != nil {
		return err
	}
	if err = m.deleteSlot(2); err != nil {
		return err
	}
	for j := m.IndexStart; j < m.Parent.Dimension(); j++ {
		if j != pj {
			if uint64(ro) >= uint64(len(m.Data)) || uint64(po) >= uint64(len(m.Data)) {
				return ErrBounds
			}
			if err = etaRationalProduct(m.manager, &m.work.Data[3], &m.work.Data[4], &m.Data[po], mult); err != nil {
				return err
			}
			// rational(row) -= product returns the owning result t.
			if err = m.manager.Copy(&m.work.Data[5], &m.Data[ro]); err != nil {
				return err
			}
			if err = m.manager.Sub(&m.work.Data[5], &m.work.Data[4], &m.work.Data[5]); err != nil {
				return err
			}
			if err = m.manager.Copy(&m.work.Data[6], &m.work.Data[5]); err != nil {
				return err
			}
			if err = m.deleteSlot(5); err != nil {
				return err
			}
			if err = m.deleteSlot(4); err != nil {
				return err
			}
			zero, e := m.manager.IsZero(&m.work.Data[6])
			if e != nil {
				return e
			}
			if zero {
				z := numeral.NewRational()
				if err = m.manager.Copy(&m.Data[ro], &z); err != nil {
					return err
				}
			} else if err = m.manager.Copy(&m.Data[ro], &m.work.Data[6]); err != nil {
				return err
			}
			if err = m.deleteSlot(6); err != nil {
				return err
			}
		}
		ro++
		po++
	}
	if err = m.deleteSlot(1); err != nil {
		return err
	}
	return m.deleteSlot(0)
}
func (m *RationalSquareDenseSubmatrix) Pivot(i uint32, s *Settings) error {
	if err := m.DivideRowByPivot(i); err != nil {
		return err
	}
	for k := i + 1; k < m.Parent.Dimension(); k++ {
		if err := m.PivotRowToRow(i, k, s); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalSquareDenseSubmatrix) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	// An interrupted workspace Add is nested inside the indexed-right call.
	// Unwind its numeric locals before destroying the argument product, while
	// retaining stored workspace cells until their member destructor below.
	if err := m.WorkVector.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if m.work != nil {
		if err := m.closeWork(); err != nil {
			return err
		}
		m.work = nil
	}
	if err := m.WorkVector.CloseForward(); err != nil {
		return err
	}
	// Source vector destruction visits coefficients forward.
	for i := range m.Data {
		if err := m.manager.Del(&m.Data[i]); err != nil {
			return err
		}
		m.Data[i] = numeral.NewRational()
	}
	m.Data = nil
	return m.calls.cleanupRoots(m.manager)
}

// A dense application constructs its vector before the nested arithmetic
// locals. Source vector destruction walks forward; lexical locals unwind back.
func (m *RationalSquareDenseSubmatrix) closeWork() error {
	if m.vectorPrefix == 0 {
		return m.work.Close()
	}
	for i := len(m.work.Data); i > int(m.vectorPrefix); {
		i--
		if err := m.deleteSlot(i); err != nil {
			return err
		}
	}
	for i := uint32(0); i < m.vectorPrefix; i++ {
		if err := m.deleteSlot(int(i)); err != nil {
			return err
		}
	}
	m.vectorPrefix = 0
	return m.work.Close()
}
