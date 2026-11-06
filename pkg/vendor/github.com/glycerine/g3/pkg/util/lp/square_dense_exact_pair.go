package lp

// Source: src/util/lp/square_dense_submatrix.hpp:175-185,273-301;
// numeric_pair.h:109-121,216-219. Retained mpq/numeric_pair dense-left path.

// PairSquareDenseSubmatrix shares rational coefficients and all scalar/right
// operations. Only the concrete pair-valued dense-left application differs.
type PairSquareDenseSubmatrix struct {
	*RationalSquareDenseSubmatrix
	pairWork         *IndexedPairVector
	pairVectorPrefix uint32
}

func NewPairSquareDenseSubmatrix(parent *RationalSparseMatrix, start uint32) (*PairSquareDenseSubmatrix, error) {
	m, err := NewRationalSquareDenseSubmatrix(parent, start)
	if m == nil {
		return nil, err
	}
	return &PairSquareDenseSubmatrix{RationalSquareDenseSubmatrix: m}, err
}
func (m *PairSquareDenseSubmatrix) closePairWork() error {
	if m.pairWork == nil {
		return nil
	}
	// Pair arithmetic locals unwind before the forward vector destructor.
	for i := len(m.pairWork.Data); i > int(m.pairVectorPrefix); {
		i--
		if err := m.pairWork.Data[i].Del(m.manager); err != nil {
			return err
		}
	}
	for i := uint32(0); i < m.pairVectorPrefix; i++ {
		if err := m.pairWork.Data[i].Del(m.manager); err != nil {
			return err
		}
	}
	if err := m.pairWork.Close(); err != nil {
		return err
	}
	m.pairWork = nil
	m.pairVectorPrefix = 0
	return nil
}
func (m *PairSquareDenseSubmatrix) pairRowProduct(i uint32, w []RationalPair, out *RationalPair) error {
	if i < m.IndexStart {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	n := m.Parent.Dimension()
	sum, product := &m.pairWork.Data[n], &m.pairWork.Data[n+1]
	x, y, tmp := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
	offset := (i - m.IndexStart) * m.Dim
	for j := uint32(0); j < m.Dim; j++ {
		col, err := m.AdjustColumnInverse(m.IndexStart + j)
		if err != nil {
			return err
		}
		if uint64(offset+j) >= uint64(len(m.Data)) || uint64(col) >= uint64(len(w)) {
			return ErrBounds
		}
		coefficient := &m.Data[offset+j]
		if err = etaRationalProduct(m.manager, tmp, x, coefficient, &w[col].X); err != nil {
			return err
		}
		if err = etaRationalProduct(m.manager, tmp, y, coefficient, &w[col].Y); err != nil {
			return err
		}
		if err = m.manager.Copy(&product.X, x); err != nil {
			return err
		}
		if err = m.manager.Copy(&product.Y, y); err != nil {
			return err
		}
		if err = m.deleteSlot(1); err != nil {
			return err
		}
		if err = m.deleteSlot(0); err != nil {
			return err
		}
		if err = sum.Add(m.manager, sum, product); err != nil {
			return err
		}
		if err = product.Del(m.manager); err != nil {
			return err
		}
	}
	if err := out.Copy(m.manager, sum); err != nil {
		return err
	}
	return sum.Del(m.manager)
}
func (m *PairSquareDenseSubmatrix) ApplyLeft(w []RationalPair) (err error) {
	b, err := m.beginWork(3)
	if err != nil {
		return err
	}
	n := m.Parent.Dimension()
	m.pairWork = NewIndexedPairVector(m.manager, n+3)
	m.pairVectorPrefix = n
	defer func() {
		err = m.calls.after(m.manager, b, err)
		if err == nil {
			err = m.closePairWork()
			if err == nil {
				err = m.closeWork()
				if err == nil {
					m.work = nil
				}
			}
		}
	}()
	t := m.pairWork.Data[:n]
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
		if err = t[row].Copy(m.manager, &w[col]); err != nil {
			return err
		}
	}
	result := &m.pairWork.Data[n+2]
	for i := m.IndexStart; i < n; i++ {
		row, e := m.AdjustRowInverse(i)
		if e != nil {
			return e
		}
		if err = m.pairRowProduct(i, w, result); err != nil {
			return err
		}
		if uint64(row) >= uint64(len(t)) {
			return ErrBounds
		}
		if err = t[row].Copy(m.manager, result); err != nil {
			return err
		}
		if err = result.Del(m.manager); err != nil {
			return err
		}
	}
	for i := uint32(0); i < n; i++ {
		if uint64(i) >= uint64(len(w)) {
			return ErrBounds
		}
		if err = w[i].Copy(m.manager, &t[i]); err != nil {
			return err
		}
	}
	return nil
}
func (m *PairSquareDenseSubmatrix) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if err := m.WorkVector.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	// Scalar construction temporaries precede the containing pair/vector owners.
	if m.work != nil {
		if err := m.closeWork(); err != nil {
			return err
		}
		m.work = nil
	}
	if err := m.closePairWork(); err != nil {
		return err
	}
	return m.RationalSquareDenseSubmatrix.Close()
}

var _ PairTailMatrix = (*PairSquareDenseSubmatrix)(nil)
