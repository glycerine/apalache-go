package lp

// Source: src/util/lp/sparse_matrix.h/.hpp and indexed_value.h, mpq storage.

import "github.com/glycerine/g3/pkg/util/numeral"

type RationalIndexedValue struct {
	Value        numeral.Rational
	Index, Other uint32
}
type RationalSparseColumn struct {
	ShortenedMarkovitz uint32
	Values             []RationalIndexedValue
}

// RationalSparseMatrix owns independent coefficients in both rows and columns.
// This coefficient storage serves the retained mpq/mpq and mpq/pair LU instances.
// Public slices and Get results are read-only borrows. Do not copy owners or keep
// element pointers across mutation. A partial constructor result must be closed.
type RationalSparseMatrix struct {
	Rows                              [][]RationalIndexedValue
	Columns                           []RationalSparseColumn
	RowPermutation, ColumnPermutation *Permutation
	WorkPivot                         []int32
	Processed                         []bool
	active                            uint32
	pivotQueue                        *UintPairPriorityQueue
	manager                           *numeral.RationalManager
	calls                             exactEtaCalls
	work                              *IndexedRationalVector
	zero                              numeral.Rational
}

func NewRationalSparseBasisMatrix(a *RationalStaticMatrix, basis []uint32) (m *RationalSparseMatrix, err error) {
	if err = a.ready(); err != nil {
		return nil, err
	}
	n := a.RowCount()
	m = &RationalSparseMatrix{
		Rows: make([][]RationalIndexedValue, n), Columns: make([]RationalSparseColumn, n),
		RowPermutation: NewPermutation(n), ColumnPermutation: NewPermutation(n),
		WorkPivot: make([]int32, n), Processed: make([]bool, n), pivotQueue: NewUintPairPriorityQueue(n),
		manager: a.manager, zero: numeral.NewRational(),
	}
	for i := range m.WorkPivot {
		m.WorkPivot[i] = -1
	}
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return m, err
	}
	m.work = NewIndexedRationalVector(m.manager, 3)
	defer func() {
		err = m.calls.after(m.manager, b, err)
		if err == nil {
			err = m.work.Close()
			if err == nil {
				m.work = nil
			}
		}
	}()
	for j := n; j > 0; {
		j--
		if uint64(j) >= uint64(len(basis)) {
			return m, ErrBounds
		}
		if err = m.copyColumnFromStatic(basis[j], a, j); err != nil {
			return m, err
		}
	}
	return m, nil
}

func (m *RationalSparseMatrix) ready() error {
	if m.work != nil || m.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}

// indexed_value takes its value by value, copies it into its member, and the
// vector copies that temporary into storage. Retain the argument through the
// full expression, then destroy temporary and argument after publication.
// Slice relocation transfers existing ownership without copying numeral handles.
func (m *RationalSparseMatrix) appendIndexed(out *[]RationalIndexedValue, value *numeral.Rational, index, other uint32) error {
	w := m.work.Data
	if err := m.manager.Copy(&w[0], value); err != nil {
		return err
	}
	if err := m.manager.Copy(&w[1], &w[0]); err != nil {
		return err
	}
	if err := m.manager.Copy(&w[2], &w[1]); err != nil {
		return err
	}
	*out = append(*out, RationalIndexedValue{w[2], index, other})
	w[2] = numeral.NewRational()
	if err := m.manager.Del(&w[1]); err != nil {
		return err
	}
	w[1] = numeral.NewRational()
	if err := m.manager.Del(&w[0]); err != nil {
		return err
	}
	w[0] = numeral.NewRational()
	return nil
}
func (m *RationalSparseMatrix) copyColumnFromStatic(column uint32, a *RationalStaticMatrix, destination uint32) error {
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
		co, ro := uint32(len(m.Columns[destination].Values)), uint32(len(m.Rows[c.Row]))
		if err = m.appendIndexed(&m.Columns[destination].Values, value, c.Row, ro); err != nil {
			return err
		}
		if err = m.appendIndexed(&m.Rows[c.Row], value, destination, co); err != nil {
			return err
		}
		m.active++
	}
	return nil
}
func (m *RationalSparseMatrix) Dimension() uint32      { return m.RowPermutation.Size() }
func (m *RationalSparseMatrix) ActiveElements() uint32 { return m.active }
func (m *RationalSparseMatrix) AdjustRow(row uint32) (uint32, error) {
	return m.RowPermutation.Value(row)
}
func (m *RationalSparseMatrix) AdjustColumn(column uint32) (uint32, error) {
	return m.ColumnPermutation.Reverse(column)
}
func (m *RationalSparseMatrix) AdjustRowInverse(row uint32) (uint32, error) {
	return m.RowPermutation.Reverse(row)
}
func (m *RationalSparseMatrix) AdjustColumnInverse(column uint32) (uint32, error) {
	return m.ColumnPermutation.Value(column)
}
func (m *RationalSparseMatrix) GetNotAdjusted(row, column uint32) (*numeral.Rational, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	if uint64(row) >= uint64(len(m.Rows)) {
		return nil, ErrBounds
	}
	for i := range m.Rows[row] {
		if m.Rows[row][i].Index == column {
			return &m.Rows[row][i].Value, nil
		}
	}
	return &m.zero, nil
}
func (m *RationalSparseMatrix) Get(row, column uint32) (*numeral.Rational, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	r, err := m.AdjustRow(row)
	if err != nil {
		return nil, err
	}
	c, err := m.AdjustColumn(column)
	if err != nil {
		return nil, err
	}
	return m.GetNotAdjusted(r, c)
}
func (m *RationalSparseMatrix) LowestRowInColumn(column uint32) (uint32, error) {
	if err := m.ready(); err != nil {
		return 0, err
	}
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
	return lowest, nil
}
func (m *RationalSparseMatrix) SwapRows(a, b uint32) error {
	if err := m.ready(); err != nil {
		return err
	}
	return m.RowPermutation.TransposeRight(a, b)
}
func (m *RationalSparseMatrix) SwapColumns(a, b uint32) error {
	if err := m.ready(); err != nil {
		return err
	}
	return m.ColumnPermutation.TransposeLeft(a, b)
}
func (m *RationalSparseMatrix) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if m.work != nil {
		if err := m.work.Close(); err != nil {
			return err
		}
		m.work = nil
	}
	// Reverse member destruction order: columns before rows. Each direction
	// owns its values even when the other direction was never published.
	for j := range m.Columns {
		for i := range m.Columns[j].Values {
			if err := m.manager.Del(&m.Columns[j].Values[i].Value); err != nil {
				return err
			}
			m.Columns[j].Values[i].Value = numeral.NewRational()
		}
	}
	for i := range m.Rows {
		for j := range m.Rows[i] {
			if err := m.manager.Del(&m.Rows[i][j].Value); err != nil {
				return err
			}
			m.Rows[i][j].Value = numeral.NewRational()
		}
	}
	m.Rows = nil
	m.Columns = nil
	m.active = 0
	return m.calls.cleanupRoots(m.manager)
}
