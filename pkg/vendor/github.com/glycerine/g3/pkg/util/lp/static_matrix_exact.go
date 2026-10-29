package lp

// Source: src/util/lp/static_matrix.h/.hpp, retained mpq coefficient storage.

import "github.com/glycerine/g3/pkg/util/numeral"

type RationalRowCell struct {
	Column, Offset uint32
	Value          numeral.Rational
}

// RationalStaticMatrix owns row coefficients in a borrowed numeral manager.
// Columns borrow row positions. Public values must not be shallow-copied, and
// pointers into rows must not be retained across mutation. Close owns stopped
// lexical values and numerical calls as well as the published coefficients.
type RationalStaticMatrix struct {
	Rows         [][]RationalRowCell
	Columns      [][]ColumnCell
	RowOffsets   []int32
	WorkVector   *IndexedRationalVector
	manager      *numeral.RationalManager
	calls        exactEtaCalls
	work         *IndexedRationalVector
	becameZeros  []uint32
	stack        []staticMatrixDimensions
	transposeRow []RationalRowCell
}

func NewRationalStaticMatrix(manager *numeral.RationalManager, rows, columns uint32) *RationalStaticMatrix {
	m := &RationalStaticMatrix{manager: manager, Rows: make([][]RationalRowCell, rows), Columns: make([][]ColumnCell, columns), WorkVector: NewIndexedRationalVector(manager, 0)}
	m.initRowOffsets()
	return m
}
func (m *RationalStaticMatrix) RowCount() uint32    { return uint32(len(m.Rows)) }
func (m *RationalStaticMatrix) ColumnCount() uint32 { return uint32(len(m.Columns)) }
func (m *RationalStaticMatrix) ready() error {
	if m.work != nil || m.calls.pending() || (m.WorkVector != nil && m.WorkVector.calls.pending()) {
		return ErrExactVectorPending
	}
	return nil
}
func (m *RationalStaticMatrix) beginWork(size uint32) (exactEtaBefore, error) {
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
func (m *RationalStaticMatrix) finishWork(b exactEtaBefore, err *error) {
	*err = m.calls.after(m.manager, b, *err)
	if *err == nil {
		*err = m.work.Close()
		if *err == nil {
			m.work = nil
		}
	}
}
func (m *RationalStaticMatrix) InitRowOffsets() error {
	if err := m.ready(); err != nil {
		return err
	}
	m.initRowOffsets()
	return nil
}
func (m *RationalStaticMatrix) initRowOffsets() {
	m.RowOffsets = make([]int32, len(m.Columns))
	for i := range m.RowOffsets {
		m.RowOffsets[i] = -1
	}
}
func (m *RationalStaticMatrix) InitRowColumns(rows, columns uint32) error {
	if err := m.ready(); err != nil {
		return err
	}
	if len(m.Rows) != 0 || len(m.Columns) != 0 {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.Rows = append(m.Rows, make([][]RationalRowCell, rows)...)
	m.Columns = append(m.Columns, make([][]ColumnCell, columns)...)
	return nil
}
func (m *RationalStaticMatrix) InitEmptyMatrix(rows, columns uint32) error {
	if err := m.ready(); err != nil {
		return err
	}
	m.initRowOffsets() // Literal old column extent, before initialization.
	return m.InitRowColumns(rows, columns)
}
func (m *RationalStaticMatrix) AddRow() error {
	if err := m.ready(); err != nil {
		return err
	}
	m.Rows = append(m.Rows, nil)
	return nil
}
func (m *RationalStaticMatrix) AddColumn() error {
	if err := m.ready(); err != nil {
		return err
	}
	m.Columns = append(m.Columns, nil)
	m.RowOffsets = append(m.RowOffsets, -1)
	return nil
}
func (m *RationalStaticMatrix) AddColumns(delta uint32) error {
	if err := m.ready(); err != nil {
		return err
	}
	for ; delta > 0; delta-- {
		if err := m.AddColumn(); err != nil {
			return err
		}
	}
	return nil
}

// appendRowValue constructs the row_cell value, then its stored copy. Native
// slice relocation transfers existing values; it does not copy numeral owners.
func (m *RationalStaticMatrix) appendRowValue(row, column, offset uint32, value *numeral.Rational, temporary, stored int) error {
	w := m.work.Data
	if err := m.manager.Copy(&w[temporary], value); err != nil {
		return err
	}
	if err := m.manager.Copy(&w[stored], &w[temporary]); err != nil {
		return err
	}
	m.Rows[row] = append(m.Rows[row], RationalRowCell{column, offset, w[stored]})
	w[stored] = numeral.NewRational()
	if err := m.manager.Del(&w[temporary]); err != nil {
		return err
	}
	w[temporary] = numeral.NewRational()
	return nil
}
func (m *RationalStaticMatrix) Set(row, column uint32, value *numeral.Rational) (err error) {
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	return m.set(row, column, value, 0, 1)
}
func (m *RationalStaticMatrix) set(row, column uint32, value *numeral.Rational, temporary, stored int) error {
	zero, err := m.manager.IsZero(value)
	if err != nil || zero {
		return err
	}
	if uint64(row) >= uint64(len(m.Rows)) || uint64(column) >= uint64(len(m.Columns)) {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	co, ro := uint32(len(m.Columns[column])), uint32(len(m.Rows[row]))
	// Source publishes this link before either exact row-cell copy.
	m.Columns[column] = append(m.Columns[column], ColumnCell{row, ro})
	return m.appendRowValue(row, column, co, value, temporary, stored)
}
func (m *RationalStaticMatrix) addNewElement(row, column uint32, value *numeral.Rational, temporary, stored int) error {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	co, ro := uint32(len(m.Columns[column])), uint32(len(m.Rows[row]))
	if err := m.appendRowValue(row, column, co, value, temporary, stored); err != nil {
		return err
	}
	m.Columns[column] = append(m.Columns[column], ColumnCell{row, ro})
	return nil
}
func (m *RationalStaticMatrix) AddNewElement(row, column uint32, value *numeral.Rational) (err error) {
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	return m.addNewElement(row, column, value, 0, 1)
}

// ColumnValue returns a borrowed coefficient, like source get_val's const ref.
func (m *RationalStaticMatrix) ColumnValue(c ColumnCell) (*numeral.Rational, error) {
	if uint64(c.Row) >= uint64(len(m.Rows)) || uint64(c.Offset) >= uint64(len(m.Rows[c.Row])) {
		return nil, ErrBounds
	}
	return &m.Rows[c.Row][c.Offset].Value, nil
}
func (m *RationalStaticMatrix) Element(row, column uint32, out *numeral.Rational) (err error) {
	b, err := m.beginWork(0)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(row) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	for i := range m.Rows[row] {
		if m.Rows[row][i].Column == column {
			return m.manager.Copy(out, &m.Rows[row][i].Value)
		}
	}
	zero := numeral.NewRational()
	return m.manager.Copy(out, &zero)
}
func (m *RationalStaticMatrix) RowSize(row uint32) (uint32, error) {
	if uint64(row) >= uint64(len(m.Rows)) {
		return 0, ErrBounds
	}
	return uint32(len(m.Rows[row])), nil
}
func (m *RationalStaticMatrix) ColumnSize(column uint32) (uint32, error) {
	if uint64(column) >= uint64(len(m.Columns)) {
		return 0, ErrBounds
	}
	return uint32(len(m.Columns[column])), nil
}
func (m *RationalStaticMatrix) Size() uint32 {
	var n uint32
	for _, row := range m.Rows {
		n += uint32(len(row))
	}
	return n
}
func (m *RationalStaticMatrix) removeElement(row, offset uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(offset) >= uint64(len(m.Rows[row])) {
		return ErrBounds
	}
	column, co := m.Rows[row][offset].Column, m.Rows[row][offset].Offset
	if uint64(column) >= uint64(len(m.Columns)) || uint64(co) >= uint64(len(m.Columns[column])) {
		return ErrBounds
	}
	col := &m.Columns[column]
	ro := (*col)[co].Offset
	if int(co) != len(*col)-1 {
		(*col)[co] = (*col)[len(*col)-1]
		c := (*col)[co]
		if uint64(c.Row) >= uint64(len(m.Rows)) || uint64(c.Offset) >= uint64(len(m.Rows[c.Row])) {
			return ErrBounds
		}
		m.Rows[c.Row][c.Offset].Offset = co
	}
	if uint64(ro) >= uint64(len(m.Rows[row])) {
		return ErrBounds
	}
	last := len(m.Rows[row]) - 1
	if int(ro) != last {
		dst, src := &m.Rows[row][ro], &m.Rows[row][last]
		// Generated row_cell assignment writes both scalar fields before the
		// owning rational assignment. A failed copy keeps that prefix.
		dst.Column, dst.Offset = src.Column, src.Offset
		if err := m.manager.Copy(&dst.Value, &src.Value); err != nil {
			return err
		}
		if uint64(dst.Column) >= uint64(len(m.Columns)) || uint64(dst.Offset) >= uint64(len(m.Columns[dst.Column])) {
			return ErrBounds
		}
		m.Columns[dst.Column][dst.Offset].Offset = ro
	}
	*col = (*col)[:len(*col)-1]
	if err := m.manager.Del(&m.Rows[row][last].Value); err != nil {
		return err
	}
	m.Rows[row][last] = RationalRowCell{}
	m.Rows[row] = m.Rows[row][:last]
	return nil
}
func (m *RationalStaticMatrix) RemoveElement(row, offset uint32) (err error) {
	b, err := m.beginWork(0)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	return m.removeElement(row, offset)
}
func (m *RationalStaticMatrix) MultiplyRow(row uint32, alpha *numeral.Rational) (err error) {
	b, err := m.beginWork(0)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(row) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	for i := range m.Rows[row] {
		v := &m.Rows[row][i].Value
		if err = m.manager.Mul(v, alpha, v); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalStaticMatrix) MultiplyColumn(column uint32, alpha *numeral.Rational) (err error) {
	b, err := m.beginWork(0)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	for _, c := range m.Columns[column] {
		v, e := m.ColumnValue(c)
		if e != nil {
			return e
		}
		if err = m.manager.Mul(v, alpha, v); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalStaticMatrix) scanRowOffsets(row uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	for j, c := range m.Rows[row] {
		if uint64(c.Column) >= uint64(len(m.RowOffsets)) {
			return ErrBounds
		}
		m.RowOffsets[c.Column] = int32(j)
	}
	return nil
}
func (m *RationalStaticMatrix) PivotRowToRow(pivot uint32, c ColumnCell, pivotColumn uint32) (nonempty bool, err error) {
	b, err := m.beginWork(5)
	if err != nil {
		return false, err
	}
	defer m.finishWork(b, &err)
	target := c.Row
	if uint64(pivot) >= uint64(len(m.Rows)) || uint64(target) >= uint64(len(m.Columns)) {
		if err = sourceAssertionFailure(); err != nil {
			return false, err
		}
	}
	if pivot == target {
		if err = sourceAssertionFailure(); err != nil {
			return false, err
		}
	}
	m.becameZeros = m.becameZeros[:0]
	value, err := m.ColumnValue(c)
	if err != nil {
		return false, err
	}
	alpha, product, expression := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
	if err = m.manager.Copy(alpha, value); err != nil {
		return false, err
	}
	if err = m.manager.Neg(alpha); err != nil {
		return false, err
	}
	zero, err := m.manager.IsZero(alpha)
	if err != nil {
		return false, err
	}
	if zero {
		if err = sourceAssertionFailure(); err != nil {
			return false, err
		}
	}
	if err = m.removeElement(target, c.Offset); err != nil {
		return false, err
	}
	if err = m.scanRowOffsets(target); err != nil {
		return false, err
	}
	previousSize := len(m.Rows[target])
	if uint64(pivot) >= uint64(len(m.Rows)) {
		return false, ErrBounds
	}
	for i := range m.Rows[pivot] {
		r := &m.Rows[pivot][i]
		if r.Column == pivotColumn {
			continue
		}
		if err = etaRationalProduct(m.manager, expression, product, alpha, &r.Value); err != nil {
			return false, err
		}
		zero, err = m.manager.IsZero(&r.Value)
		if err != nil {
			return false, err
		}
		if zero {
			if err = sourceAssertionFailure(); err != nil {
				return false, err
			}
		}
		column := r.Column
		if uint64(column) >= uint64(len(m.RowOffsets)) {
			return false, ErrBounds
		}
		offset := m.RowOffsets[column]
		if offset == -1 {
			if err = m.addNewElement(target, column, product, 3, 4); err != nil {
				return false, err
			}
		} else {
			if offset < 0 || int64(offset) >= int64(len(m.Rows[target])) {
				return false, ErrBounds
			}
			v := &m.Rows[target][offset].Value
			if err = m.manager.Add(v, product, v); err != nil {
				return false, err
			}
			zero, err = m.manager.IsZero(v)
			if err != nil {
				return false, err
			}
			if zero {
				m.becameZeros = append(m.becameZeros, uint32(offset))
				for j := len(m.becameZeros) - 1; j > 0; j-- {
					if m.becameZeros[j] > m.becameZeros[j-1] {
						break
					}
					m.becameZeros[j], m.becameZeros[j-1] = m.becameZeros[j-1], m.becameZeros[j]
				}
			}
		}
		if err = m.manager.Del(product); err != nil {
			return false, err
		}
		*product = numeral.NewRational()
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
		if err = m.removeElement(target, m.becameZeros[k]); err != nil {
			return false, err
		}
	}
	return len(m.Rows[target]) != 0, nil
}
func (m *RationalStaticMatrix) Clear() error {
	if err := m.ready(); err != nil {
		return err
	}
	return m.clearRows()
}
func (m *RationalStaticMatrix) clearRows() error {
	for len(m.Rows) > 0 {
		i := len(m.Rows) - 1
		for len(m.Rows[i]) > 0 {
			j := len(m.Rows[i]) - 1
			if err := m.manager.Del(&m.Rows[i][j].Value); err != nil {
				return err
			}
			m.Rows[i][j] = RationalRowCell{}
			m.Rows[i] = m.Rows[i][:j]
		}
		m.Rows = m.Rows[:i]
	}
	m.Rows = nil
	m.Columns = nil
	m.RowOffsets = nil
	return nil
}
func (m *RationalStaticMatrix) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if m.work != nil {
		if err := m.work.Close(); err != nil {
			return err
		}
		m.work = nil
	}
	if m.WorkVector != nil {
		if err := m.WorkVector.CloseForward(); err != nil {
			return err
		}
	}
	if err := m.clearOwnedRow(&m.transposeRow); err != nil {
		return err
	}
	if err := m.clearRows(); err != nil {
		return err
	}
	m.stack = nil
	m.becameZeros = nil
	return m.calls.cleanupRoots(m.manager)
}
