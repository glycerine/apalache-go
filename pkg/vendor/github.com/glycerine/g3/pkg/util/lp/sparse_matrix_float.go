package lp

// Source: src/util/lp/sparse_matrix.h/.hpp, retained double/double basis storage.

// FloatIndexedValue owns a coefficient; Index names the opposite dimension and
// Other is the reciprocal offset. Sparse matrices own values in BOTH directions,
// unlike static matrices, whose column cells only point into rows.
type FloatIndexedValue struct {
	Value        float64
	Index, Other uint32
}

type FloatSparseColumn struct {
	ShortenedMarkovitz uint32
	Values             []FloatIndexedValue
}

// FloatSparseMatrix is the square basis workspace consumed by LU. Public slices
// are borrowed. Permutations map logical rows forward and columns in reverse.
// Construction visits basis columns backwards and retains duplicate entries.
// LU factorization and column updates consume this workspace.
type FloatSparseMatrix struct {
	Rows                              [][]FloatIndexedValue
	Columns                           []FloatSparseColumn
	RowPermutation, ColumnPermutation *Permutation
	WorkPivot                         []int32
	Processed                         []bool
	active                            uint32
	pivotQueue                        *UintPairPriorityQueue
}

func newFloatSparseMatrix(n uint32) *FloatSparseMatrix {
	m := &FloatSparseMatrix{
		Rows: make([][]FloatIndexedValue, n), Columns: make([]FloatSparseColumn, n),
		RowPermutation: NewPermutation(n), ColumnPermutation: NewPermutation(n),
		WorkPivot: make([]int32, n), Processed: make([]bool, n),
		pivotQueue: NewUintPairPriorityQueue(n),
	}
	for i := range m.WorkPivot {
		m.WorkPivot[i] = -1
	}
	return m
}

// NewFloatSparseBasisMatrix copies A[:,basis] with independent row and column
// coefficients. On error, the returned owner retains the completed prefix.
func NewFloatSparseBasisMatrix(a *FloatStaticMatrix, basis []uint32) (*FloatSparseMatrix, error) {
	m := newFloatSparseMatrix(a.RowCount())
	for j := a.RowCount(); j > 0; {
		j--
		if uint64(j) >= uint64(len(basis)) {
			return m, ErrBounds
		}
		if err := m.copyColumnFromStatic(basis[j], a, j); err != nil {
			return m, err
		}
	}
	return m, nil
}

func (m *FloatSparseMatrix) copyColumnFromStatic(column uint32, a *FloatStaticMatrix, destination uint32) error {
	if uint64(column) >= uint64(len(a.Columns)) || uint64(destination) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	for _, c := range a.Columns[column] {
		if uint64(c.Row) >= uint64(len(m.Rows)) {
			return ErrBounds
		}
		value, err := a.ColumnValue(c)
		if err != nil {
			return err
		}
		co := uint32(len(m.Columns[destination].Values))
		ro := uint32(len(m.Rows[c.Row]))
		m.Columns[destination].Values = append(m.Columns[destination].Values, FloatIndexedValue{value, c.Row, ro})
		m.Rows[c.Row] = append(m.Rows[c.Row], FloatIndexedValue{value, destination, co})
		m.active++
	}
	return nil
}

func (m *FloatSparseMatrix) Dimension() uint32      { return m.RowPermutation.Size() }
func (m *FloatSparseMatrix) ActiveElements() uint32 { return m.active }
func (m *FloatSparseMatrix) AdjustRow(row uint32) (uint32, error) {
	return m.RowPermutation.Value(row)
}
func (m *FloatSparseMatrix) AdjustColumn(column uint32) (uint32, error) {
	return m.ColumnPermutation.Reverse(column)
}
func (m *FloatSparseMatrix) AdjustRowInverse(row uint32) (uint32, error) {
	return m.RowPermutation.Reverse(row)
}
func (m *FloatSparseMatrix) AdjustColumnInverse(column uint32) (uint32, error) {
	return m.ColumnPermutation.Value(column)
}
func (m *FloatSparseMatrix) GetNotAdjusted(row, column uint32) (float64, error) {
	if uint64(row) >= uint64(len(m.Rows)) {
		return 0, ErrBounds
	}
	for _, v := range m.Rows[row] {
		if v.Index == column {
			return v.Value, nil
		}
	}
	return 0, nil
}
func (m *FloatSparseMatrix) Get(row, column uint32) (float64, error) {
	r, err := m.AdjustRow(row)
	if err != nil {
		return 0, err
	}
	c, err := m.AdjustColumn(column)
	if err != nil {
		return 0, err
	}
	return m.GetNotAdjusted(r, c)
}
func (m *FloatSparseMatrix) LowestRowInColumn(column uint32) (uint32, error) {
	c, err := m.AdjustColumn(column)
	if err != nil {
		return 0, err
	}
	if uint64(c) >= uint64(len(m.Columns)) {
		return 0, ErrBounds
	}
	var lowest uint32
	for _, v := range m.Columns[c].Values {
		r, err := m.AdjustRowInverse(v.Index)
		if err != nil {
			return 0, err
		}
		if r > lowest {
			lowest = r
		}
	}
	return lowest, nil // An empty column returns zero, literally as in source.
}

func (m *FloatSparseMatrix) SwapRows(a, b uint32) error {
	return m.RowPermutation.TransposeRight(a, b)
}
func (m *FloatSparseMatrix) SwapColumns(a, b uint32) error {
	return m.ColumnPermutation.TransposeLeft(a, b)
}
