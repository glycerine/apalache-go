package lp

// Source: src/util/lp/static_matrix.h/.hpp, retained double/double operations.

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
)

// ColumnCell points at the owning value in a matrix row. The row cell points
// back at this column position. Both offsets participate in pivot removal.
type ColumnCell struct{ Row, Offset uint32 }
type FloatRowCell struct {
	Column, Offset uint32
	Value          float64
}
type staticMatrixDimensions struct{ rows, columns uint32 }

// FloatStaticMatrix retains sparse terms in literal insertion order, with
// linked row/column positions. Set appends, rather than replacing duplicates.
// Public storage is borrowed; do not hold element pointers across mutation.
type FloatStaticMatrix struct {
	Rows        [][]FloatRowCell
	Columns     [][]ColumnCell
	RowOffsets  []int32
	stack       []staticMatrixDimensions
	becameZeros []uint32
}

func NewFloatStaticMatrix(rows, columns uint32) *FloatStaticMatrix {
	m := &FloatStaticMatrix{Rows: make([][]FloatRowCell, rows), Columns: make([][]ColumnCell, columns), RowOffsets: make([]int32, columns)}
	for i := range m.RowOffsets {
		m.RowOffsets[i] = -1
	}
	return m
}
func (m *FloatStaticMatrix) RowCount() uint32    { return uint32(len(m.Rows)) }
func (m *FloatStaticMatrix) ColumnCount() uint32 { return uint32(len(m.Columns)) }
func (m *FloatStaticMatrix) Clear()              { m.Rows = nil; m.Columns = nil; m.RowOffsets = nil }
func (m *FloatStaticMatrix) InitRowColumns(rows, columns uint32) error {
	if len(m.Rows) != 0 || len(m.Columns) != 0 {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.Rows = append(m.Rows, make([][]FloatRowCell, rows)...)
	m.Columns = append(m.Columns, make([][]ColumnCell, columns)...)
	return nil
}
func (m *FloatStaticMatrix) InitRowOffsets() {
	m.RowOffsets = make([]int32, len(m.Columns))
	for i := range m.RowOffsets {
		m.RowOffsets[i] = -1
	}
}
func (m *FloatStaticMatrix) InitEmptyMatrix(rows, columns uint32) error {
	// Source sizes offsets from the OLD column count, before adding columns.
	m.InitRowOffsets()
	return m.InitRowColumns(rows, columns)
}
func (m *FloatStaticMatrix) AddRow() { m.Rows = append(m.Rows, nil) }
func (m *FloatStaticMatrix) AddColumn() {
	m.Columns = append(m.Columns, nil)
	m.RowOffsets = append(m.RowOffsets, -1)
}
func (m *FloatStaticMatrix) AddColumns(delta uint32) {
	for ; delta > 0; delta-- {
		m.AddColumn()
	}
}
func (m *FloatStaticMatrix) Set(row, column uint32, value float64) error {
	if value == 0 {
		return nil
	}
	if uint64(row) >= uint64(len(m.Rows)) || uint64(column) >= uint64(len(m.Columns)) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	co, ro := uint32(len(m.Columns[column])), uint32(len(m.Rows[row]))
	m.Columns[column] = append(m.Columns[column], ColumnCell{row, ro})
	m.Rows[row] = append(m.Rows[row], FloatRowCell{column, co, value})
	return nil
}
func (m *FloatStaticMatrix) AddNewElement(row, column uint32, value float64) error {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	ro, co := uint32(len(m.Rows[row])), uint32(len(m.Columns[column]))
	m.Rows[row] = append(m.Rows[row], FloatRowCell{column, co, value})
	m.Columns[column] = append(m.Columns[column], ColumnCell{row, ro})
	return nil
}
func (m *FloatStaticMatrix) ColumnValue(c ColumnCell) (float64, error) {
	if uint64(c.Row) >= uint64(len(m.Rows)) || uint64(c.Offset) >= uint64(len(m.Rows[c.Row])) {
		return 0, ErrBounds
	}
	return m.Rows[c.Row][c.Offset].Value, nil
}
func (m *FloatStaticMatrix) Element(row, column uint32) (float64, error) {
	if uint64(row) >= uint64(len(m.Rows)) {
		return 0, ErrBounds
	}
	for _, c := range m.Rows[row] {
		if c.Column == column {
			return c.Value, nil
		}
	}
	return 0, nil
}
func (m *FloatStaticMatrix) Domain() []IndexPair {
	seen := make(map[IndexPair]bool)
	var out []IndexPair
	for i, row := range m.Rows {
		for _, c := range row {
			p := IndexPair{uint32(i), c.Column}
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].I < out[j].I || out[i].I == out[j].I && out[i].J < out[j].J })
	return out
}
func (m *FloatStaticMatrix) RowSize(row uint32) (uint32, error) {
	if uint64(row) >= uint64(len(m.Rows)) {
		return 0, ErrBounds
	}
	return uint32(len(m.Rows[row])), nil
}
func (m *FloatStaticMatrix) ColumnSize(column uint32) (uint32, error) {
	if uint64(column) >= uint64(len(m.Columns)) {
		return 0, ErrBounds
	}
	return uint32(len(m.Columns[column])), nil
}
func (m *FloatStaticMatrix) Size() uint32 {
	var n uint32
	for _, r := range m.Rows {
		n += uint32(len(r))
	}
	return n
}
func (m *FloatStaticMatrix) CopyColumnToIndexedVector(column uint32, v *IndexedFloatVector) error {
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
		if value != 0 {
			if err = v.SetValue(value, c.Row); err != nil {
				return err
			}
		}
	}
	return nil
}
func (m *FloatStaticMatrix) CopyColumnToVector(column uint32, v []float64) ([]float64, error) {
	if len(v) > len(m.Rows) {
		v = v[:len(m.Rows)]
	} else {
		v = append(v, make([]float64, len(m.Rows)-len(v))...)
	}
	if uint64(column) >= uint64(len(m.Columns)) {
		return v, ErrBounds
	}
	for _, c := range m.Columns[column] {
		value, err := m.ColumnValue(c)
		if err != nil {
			return v, err
		}
		if value != 0 {
			if uint64(c.Row) >= uint64(len(v)) {
				return v, ErrBounds
			}
			v[c.Row] = value
		}
	}
	return v, nil
}
func (m *FloatStaticMatrix) AddColumnToVector(alpha float64, column uint32, v []float64) error {
	if uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	for _, c := range m.Columns[column] {
		if uint64(c.Row) >= uint64(len(v)) {
			return ErrBounds
		}
		value, err := m.ColumnValue(c)
		if err != nil {
			return err
		}
		v[c.Row] += alpha * value
	}
	return nil
}
func (m *FloatStaticMatrix) DotColumn(v []float64, column uint32) (float64, error) {
	if uint64(column) >= uint64(len(m.Columns)) {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
		return 0, ErrBounds
	}
	var result float64
	for _, c := range m.Columns[column] {
		if uint64(c.Row) >= uint64(len(v)) {
			return result, ErrBounds
		}
		value, err := m.ColumnValue(c)
		if err != nil {
			return result, err
		}
		result += v[c.Row] * value
	}
	return result, nil
}
func (m *FloatStaticMatrix) DotRow(row uint32, v []float64) (float64, error) {
	if uint64(row) >= uint64(len(m.Rows)) {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
		return 0, ErrBounds
	}
	var result float64
	for _, c := range m.Rows[row] {
		if uint64(c.Column) >= uint64(len(v)) {
			return result, ErrBounds
		}
		result += v[c.Column] * c.Value
	}
	return result, nil
}
func (m *FloatStaticMatrix) MultiplyRow(row uint32, alpha float64) error {
	if uint64(row) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	for i := range m.Rows[row] {
		m.Rows[row][i].Value *= alpha
	}
	return nil
}
func (m *FloatStaticMatrix) MultiplyColumn(column uint32, alpha float64) error {
	if uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	for _, c := range m.Columns[column] {
		if _, err := m.ColumnValue(c); err != nil {
			return err
		}
		m.Rows[c.Row][c.Offset].Value *= alpha
	}
	return nil
}
func (m *FloatStaticMatrix) MaxAbsRow(row uint32) (float64, error) {
	if uint64(row) >= uint64(len(m.Rows)) {
		return 0, ErrBounds
	}
	var result float64
	for _, c := range m.Rows[row] {
		a := math.Abs(c.Value)
		if a > result {
			result = a
		}
	}
	return result, nil
}
func (m *FloatStaticMatrix) MinAbsRow(row uint32) (float64, error) {
	if uint64(row) >= uint64(len(m.Rows)) {
		return 0, ErrBounds
	}
	var result float64
	for i, c := range m.Rows[row] {
		a := math.Abs(c.Value)
		if i == 0 || a < result {
			result = a
		}
	}
	return result, nil
}
func (m *FloatStaticMatrix) MaxAbsColumn(column uint32) (float64, error) {
	if uint64(column) >= uint64(len(m.Columns)) {
		return 0, ErrBounds
	}
	var result float64
	for _, c := range m.Columns[column] {
		v, err := m.ColumnValue(c)
		if err != nil {
			return result, err
		}
		a := math.Abs(v)
		if a > result {
			result = a
		}
	}
	return result, nil
}
func (m *FloatStaticMatrix) MinAbsColumn(column uint32) (float64, error) {
	if uint64(column) >= uint64(len(m.Columns)) {
		return 0, ErrBounds
	}
	var result float64
	for i, c := range m.Columns[column] {
		v, err := m.ColumnValue(c)
		if err != nil {
			return result, err
		}
		a := math.Abs(v)
		if i == 0 || a < result {
			result = a
		}
	}
	return result, nil
}

// The source numeric_traits<double>::log reaches NOT_IMPLEMENTED_YET and exits;
// it supplies no balance algorithm. Preserve that stop in the Go error profile.
var ErrLPBalanceNotImplemented = errors.New("lp: source balance logarithm is not implemented")

func (m *FloatStaticMatrix) RowBalance(row uint32) (float64, error) {
	if uint64(row) >= uint64(len(m.Rows)) {
		return 0, ErrBounds
	}
	for _, c := range m.Rows[row] {
		if c.Value != 0 {
			return 0, ErrLPBalanceNotImplemented
		}
	}
	return 0, nil
}
func (m *FloatStaticMatrix) Balance() (float64, error) {
	for i := range m.Rows {
		if _, err := m.RowBalance(uint32(i)); err != nil {
			return 0, err
		}
	}
	return 0, nil
}
func (m *FloatStaticMatrix) Push() {
	m.stack = append(m.stack, staticMatrixDimensions{m.RowCount(), m.ColumnCount()})
}
func (m *FloatStaticMatrix) Pop(count uint32) error {
	for ; count > 0; count-- {
		if len(m.stack) == 0 {
			break
		}
		d := m.stack[len(m.stack)-1]
		for uint64(d.rows) < uint64(len(m.Rows)) {
			i := len(m.Rows) - 1
			for _, r := range m.Rows[i] {
				if uint64(r.Column) >= uint64(len(m.Columns)) || len(m.Columns[r.Column]) == 0 {
					return ErrBounds
				}
				col := m.Columns[r.Column]
				if col[len(col)-1].Row != uint32(i) {
					if err := sourceAssertionFailure(); err != nil {
						return err
					}
				}
				m.Columns[r.Column] = col[:len(col)-1]
			}
			m.Rows = m.Rows[:i]
		}
		for uint64(d.columns) < uint64(len(m.Columns)) {
			m.Columns = m.Columns[:len(m.Columns)-1]
		}
		m.stack = m.stack[:len(m.stack)-1]
	}
	if sourceDebugEnabled {
		ok, err := m.IsCorrect(nil)
		if err != nil {
			return err
		}
		if !ok {
			return sourceAssertionFailure()
		}
	}
	return nil
}
func (m *FloatStaticMatrix) TransposeRows(i, j uint32) error {
	if uint64(i) >= uint64(len(m.Rows)) || uint64(j) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	m.Rows[i], m.Rows[j] = m.Rows[j], m.Rows[i]
	for _, r := range m.Rows[i] {
		if uint64(r.Column) >= uint64(len(m.Columns)) || uint64(r.Offset) >= uint64(len(m.Columns[r.Column])) {
			return ErrBounds
		}
		c := &m.Columns[r.Column][r.Offset]
		if c.Row != j {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
		}
		c.Row = i
	}
	for _, r := range m.Rows[j] {
		if uint64(r.Column) >= uint64(len(m.Columns)) || uint64(r.Offset) >= uint64(len(m.Columns[r.Column])) {
			return ErrBounds
		}
		c := &m.Columns[r.Column][r.Offset]
		if c.Row != i {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
		}
		c.Row = j
	}
	return nil
}
func (m *FloatStaticMatrix) RemoveElement(row, offset uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(offset) >= uint64(len(m.Rows[row])) {
		return ErrBounds
	}
	r := m.Rows[row][offset]
	if uint64(r.Column) >= uint64(len(m.Columns)) || uint64(r.Offset) >= uint64(len(m.Columns[r.Column])) {
		return ErrBounds
	}
	col := &m.Columns[r.Column]
	ro := (*col)[r.Offset].Offset
	if int(r.Offset) != len(*col)-1 {
		(*col)[r.Offset] = (*col)[len(*col)-1]
		c := (*col)[r.Offset]
		if uint64(c.Row) >= uint64(len(m.Rows)) || uint64(c.Offset) >= uint64(len(m.Rows[c.Row])) {
			return ErrBounds
		}
		m.Rows[c.Row][c.Offset].Offset = r.Offset
	}
	if uint64(ro) >= uint64(len(m.Rows[row])) {
		return ErrBounds
	}
	if int(ro) != len(m.Rows[row])-1 {
		m.Rows[row][ro] = m.Rows[row][len(m.Rows[row])-1]
		tail := m.Rows[row][ro]
		if uint64(tail.Column) >= uint64(len(m.Columns)) || uint64(tail.Offset) >= uint64(len(m.Columns[tail.Column])) {
			return ErrBounds
		}
		m.Columns[tail.Column][tail.Offset].Offset = ro
	}
	*col = (*col)[:len(*col)-1]
	m.Rows[row] = m.Rows[row][:len(m.Rows[row])-1]
	return nil
}
func (m *FloatStaticMatrix) scanRowOffsets(row uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	for j, r := range m.Rows[row] {
		if uint64(r.Column) >= uint64(len(m.RowOffsets)) {
			return ErrBounds
		}
		m.RowOffsets[r.Column] = int32(j)
	}
	return nil
}
func (m *FloatStaticMatrix) PivotRowToRow(pivot uint32, c ColumnCell, pivotColumn uint32) (bool, error) {
	target := c.Row
	if uint64(pivot) >= uint64(len(m.Rows)) || uint64(target) >= uint64(len(m.Columns)) {
		if err := sourceAssertionFailure(); err != nil {
			return false, err
		}
	}
	if pivot == target {
		if err := sourceAssertionFailure(); err != nil {
			return false, err
		}
	}
	m.becameZeros = m.becameZeros[:0]
	value, err := m.ColumnValue(c)
	if err != nil {
		return false, err
	}
	alpha := -value
	if alpha == 0 {
		if err = sourceAssertionFailure(); err != nil {
			return false, err
		}
	}
	if err = m.RemoveElement(target, c.Offset); err != nil {
		return false, err
	}
	if err = m.scanRowOffsets(target); err != nil {
		return false, err
	}
	previousSize := len(m.Rows[target])
	if uint64(pivot) >= uint64(len(m.Rows)) {
		return false, ErrBounds
	}
	for _, r := range m.Rows[pivot] {
		if r.Column == pivotColumn {
			continue
		}
		v := alpha * r.Value
		if r.Value == 0 {
			if err = sourceAssertionFailure(); err != nil {
				return false, err
			}
		}
		if uint64(r.Column) >= uint64(len(m.RowOffsets)) {
			return false, ErrBounds
		}
		offset := m.RowOffsets[r.Column]
		if offset == -1 {
			if err = m.AddNewElement(target, r.Column, v); err != nil {
				return false, err
			}
		} else {
			if offset < 0 || int64(offset) >= int64(len(m.Rows[target])) {
				return false, ErrBounds
			}
			m.Rows[target][offset].Value += v
			if m.Rows[target][offset].Value == 0 {
				m.becameZeros = append(m.becameZeros, uint32(offset))
				for j := len(m.becameZeros) - 1; j > 0; j-- {
					if m.becameZeros[j] > m.becameZeros[j-1] {
						break
					}
					m.becameZeros[j], m.becameZeros[j-1] = m.becameZeros[j-1], m.becameZeros[j]
				}
			}
		}
	}
	for k := 0; k < previousSize; k++ {
		column := m.Rows[target][k].Column
		if uint64(column) >= uint64(len(m.RowOffsets)) {
			return false, ErrBounds
		}
		m.RowOffsets[column] = -1
	}
	for k := len(m.becameZeros); k > 0; {
		k--
		if err = m.RemoveElement(target, m.becameZeros[k]); err != nil {
			return false, err
		}
	}
	return len(m.Rows[target]) != 0, nil
}
func (m *FloatStaticMatrix) IsCorrect(out io.Writer) (bool, error) {
	if out == nil {
		out = os.Stdout
	}
	for _, row := range m.Rows {
		seen := make(map[uint32]bool)
		for _, r := range row {
			if seen[r.Column] {
				_, err := fmt.Fprintf(out, "found column %d twice in a row\n", r.Column)
				return false, err
			}
			seen[r.Column] = true
			if uint64(r.Column) >= uint64(len(m.Columns)) || uint64(r.Offset) >= uint64(len(m.Columns[r.Column])) {
				return false, nil
			}
			value, err := m.ColumnValue(m.Columns[r.Column][r.Offset])
			if err != nil {
				return false, err
			}
			if r.Value != value {
				return false, nil
			}
		}
	}
	for _, column := range m.Columns {
		seen := make(map[uint32]bool)
		for _, c := range column {
			if seen[c.Row] {
				_, err := fmt.Fprintf(out, "found row %d twice in a column\n", c.Row)
				return false, err
			}
			seen[c.Row] = true
			if uint64(c.Row) >= uint64(len(m.Rows)) || uint64(c.Offset) >= uint64(len(m.Rows[c.Row])) {
				return false, nil
			}
			// Source compares get_val(c) with that very same row value. Keep
			// this diagnostic gap; independent tests check actual back-links.
			value, err := m.ColumnValue(c)
			if err != nil {
				return false, err
			}
			if value != m.Rows[c.Row][c.Offset].Value {
				return false, nil
			}
		}
	}
	return true, nil
}
