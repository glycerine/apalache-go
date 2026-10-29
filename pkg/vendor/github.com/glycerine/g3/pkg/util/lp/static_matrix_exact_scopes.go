package lp

// Source: src/util/lp/static_matrix.h/.hpp, exact scopes/transposition/diagnostics.

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (m *RationalStaticMatrix) Push() error {
	if err := m.ready(); err != nil {
		return err
	}
	m.stack = append(m.stack, staticMatrixDimensions{m.RowCount(), m.ColumnCount()})
	return nil
}
func (m *RationalStaticMatrix) Pop(count uint32) (err error) {
	b, err := m.beginWork(0)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
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
					if err = sourceAssertionFailure(); err != nil {
						return err
					}
				}
				m.Columns[r.Column] = col[:len(col)-1]
			}
			if err = m.clearOwnedRow(&m.Rows[i]); err != nil {
				return err
			}
			m.Rows = m.Rows[:i]
		}
		for uint64(d.columns) < uint64(len(m.Columns)) {
			m.Columns = m.Columns[:len(m.Columns)-1]
		}
		m.stack = m.stack[:len(m.stack)-1]
	}
	// Source does not truncate row-offset workspace or restore old values.
	if sourceDebugEnabled {
		ok, e := m.isCorrect(nil)
		if e != nil {
			return e
		}
		if !ok {
			return sourceAssertionFailure()
		}
	}
	return nil
}

// clearOwnedRow releases row-cell numerals in source vector destruction order.
// Initialized zeros retain completed deletion prefixes if a child guard stops.
func (m *RationalStaticMatrix) clearOwnedRow(row *[]RationalRowCell) error {
	for i := range *row {
		if err := m.manager.Del(&(*row)[i].Value); err != nil {
			return err
		}
		(*row)[i].Value = numeral.NewRational()
	}
	*row = nil
	return nil
}

// copyTransposeRow is local to the three reached row-value copies, rather
// than a C++ vector implementation. Native allocation initializes safe owning
// zeros; only actual coefficient copies can reserve numeral storage.
func (m *RationalStaticMatrix) copyTransposeRow(dst *[]RationalRowCell, src []RationalRowCell) error {
	if err := m.clearOwnedRow(dst); err != nil {
		return err
	}
	*dst = make([]RationalRowCell, len(src))
	for i := range *dst {
		(*dst)[i].Value = numeral.NewRational()
	}
	for i := range src {
		(*dst)[i].Column, (*dst)[i].Offset = src[i].Column, src[i].Offset
		if err := m.manager.Copy(&(*dst)[i].Value, &src[i].Value); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalStaticMatrix) TransposeRows(i, j uint32) (err error) {
	b, err := m.beginWork(0)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(i) >= uint64(len(m.Rows)) || uint64(j) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	if err = m.copyTransposeRow(&m.transposeRow, m.Rows[i]); err != nil {
		return err
	}
	// The source's first assignment is a self-assignment no-op when i == j.
	if i != j {
		if err = m.copyTransposeRow(&m.Rows[i], m.Rows[j]); err != nil {
			return err
		}
	}
	if err = m.copyTransposeRow(&m.Rows[j], m.transposeRow); err != nil {
		return err
	}
	for _, r := range m.Rows[i] {
		if uint64(r.Column) >= uint64(len(m.Columns)) || uint64(r.Offset) >= uint64(len(m.Columns[r.Column])) {
			return ErrBounds
		}
		c := &m.Columns[r.Column][r.Offset]
		if c.Row != j {
			if err = sourceAssertionFailure(); err != nil {
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
			if err = sourceAssertionFailure(); err != nil {
				return err
			}
		}
		c.Row = j
	}
	return m.clearOwnedRow(&m.transposeRow)
}
func (m *RationalStaticMatrix) Domain() []IndexPair {
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
func (m *RationalStaticMatrix) IsCorrect(out io.Writer) (bool, error) {
	if err := m.ready(); err != nil {
		return false, err
	}
	return m.isCorrect(out)
}
func (m *RationalStaticMatrix) isCorrect(out io.Writer) (bool, error) {
	if out == nil {
		out = os.Stdout
	}
	for _, row := range m.Rows {
		seen := make(map[uint32]bool)
		for i := range row {
			r := &row[i]
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
			equal, err := m.manager.Equal(&r.Value, value)
			if err != nil {
				return false, err
			}
			if !equal {
				return false, nil
			}
		}
	}
	for _, col := range m.Columns {
		seen := make(map[uint32]bool)
		for _, c := range col {
			if seen[c.Row] {
				_, err := fmt.Fprintf(out, "found row %d twice in a column\n", c.Row)
				return false, err
			}
			seen[c.Row] = true
			if uint64(c.Row) >= uint64(len(m.Rows)) || uint64(c.Offset) >= uint64(len(m.Rows[c.Row])) {
				return false, nil
			}
			value, err := m.ColumnValue(c)
			if err != nil {
				return false, err
			}
			// Source compares this coefficient with itself, leaving the diagnostic
			// backlink gap. Independent tests check the actual reciprocal coordinates.
			equal, err := m.manager.Equal(value, &m.Rows[c.Row][c.Offset].Value)
			if err != nil {
				return false, err
			}
			if !equal {
				return false, nil
			}
		}
	}
	return true, nil
}
