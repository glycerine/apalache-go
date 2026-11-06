//go:build g3debug

package lp

// Source: src/util/lp/dense_matrix.h/.hpp; retained double/double debug consumers.

// FloatDebugMatrix is the concrete double-valued source debug matrix interface.
// Existing static, permutation, eta and square-dense matrices implement it.
type FloatDebugMatrix interface {
	RowCount() uint32
	ColumnCount() uint32
	Element(uint32, uint32) (float64, error)
}

// FloatDenseMatrix owns native Go scalar storage. Source rows*columns and
// flattened offsets use unsigned32 arithmetic, including its wrap behavior.
// Go reports ErrBounds before a source out-of-bounds access. Do not shallow-copy.
type FloatDenseMatrix struct {
	rows, columns uint32
	values        []float64
}

func NewFloatDenseMatrix(rows, columns uint32) (*FloatDenseMatrix, error) {
	count := rows * columns
	if uint64(count) > uint64(^uint(0)>>1) {
		return nil, ErrBounds
	}
	return &FloatDenseMatrix{rows: rows, columns: columns, values: make([]float64, int(count))}, nil
}
func (m *FloatDenseMatrix) RowCount() uint32    { return m.rows }
func (m *FloatDenseMatrix) ColumnCount() uint32 { return m.columns }
func (m *FloatDenseMatrix) Element(i, j uint32) (float64, error) {
	offset := i*m.columns + j
	if uint64(offset) >= uint64(len(m.values)) {
		return 0, ErrBounds
	}
	return m.values[offset], nil
}
func (m *FloatDenseMatrix) SetElement(i, j uint32, value float64) error {
	offset := i*m.columns + j
	if uint64(offset) >= uint64(len(m.values)) {
		return ErrBounds
	}
	m.values[offset] = value
	return nil
}
func CopyFloatDenseMatrix(other FloatDebugMatrix) (*FloatDenseMatrix, error) {
	if other == nil {
		return nil, ErrBounds
	}
	m, err := NewFloatDenseMatrix(other.RowCount(), other.ColumnCount())
	if err != nil {
		return m, err
	}
	for i := uint32(0); i < m.rows; i++ {
		for j := uint32(0); j < m.columns; j++ {
			value, err := other.Element(i, j)
			if err != nil {
				return m, err
			}
			if err = m.SetElement(i, j, value); err != nil {
				return m, err
			}
		}
	}
	return m, nil
}

// Assign implements dense-to-dense assignment, whose dimensions publish before
// backing resize/copy. The unused base-matrix assignment is not represented.
func (m *FloatDenseMatrix) Assign(other *FloatDenseMatrix) error {
	if other == nil {
		return ErrBounds
	}
	if m == other {
		return nil
	}
	m.rows, m.columns = other.rows, other.columns
	count := m.rows * m.columns
	if uint64(count) > uint64(^uint(0)>>1) {
		return ErrBounds
	}
	if int(count) <= cap(m.values) {
		m.values = m.values[:int(count)]
	} else {
		m.values = make([]float64, int(count))
	}
	for i := uint32(0); i < m.rows; i++ {
		for j := uint32(0); j < m.columns; j++ {
			value, err := other.Element(i, j)
			if err != nil {
				return err
			}
			if err = m.SetElement(i, j, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// ApplyLeft retains the source square row_count loop even for rectangular
// matrices. It computes all temporary values before publishing w's prefix.
func (m *FloatDenseMatrix) ApplyLeft(w []float64) error {
	if uint64(m.rows) > uint64(^uint(0)>>1) {
		return ErrBounds
	}
	tmp := make([]float64, int(m.rows))
	for i := uint32(0); i < m.rows; i++ {
		value := 0.0
		for j := uint32(0); j < m.rows; j++ {
			if uint64(j) >= uint64(len(w)) {
				return ErrBounds
			}
			a, err := m.Element(i, j)
			if err != nil {
				return err
			}
			value += w[j] * a
		}
		tmp[i] = value
	}
	for i := uint32(0); i < m.rows; i++ {
		if uint64(i) >= uint64(len(w)) {
			return ErrBounds
		}
		w[i] = tmp[i]
	}
	return nil
}
