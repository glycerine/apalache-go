//go:build g3debug

package lp

// Source: src/util/lp/matrix.hpp:75-129; lu.hpp:48-83; lp_settings.h:377-379.

import "io"

func DebugMatrixColumnWidth(column uint32, rows [][]string) (uint32, error) {
	width := uint32(0)
	for _, row := range rows {
		if uint64(column) >= uint64(len(row)) {
			return width, ErrBounds
		}
		size := uint32(len(row[column]))
		if size > width {
			width = size
		}
	}
	return width, nil
}
func debugPrintBlanks(n int32, out io.Writer) error {
	// Source while(n--) has no defined terminating behavior for negative n.
	if n < 0 || out == nil {
		return ErrBounds
	}
	for n > 0 {
		if err := writeConstraintText(out, " "); err != nil {
			return err
		}
		n--
	}
	return nil
}

// PrintDebugMatrixWithWidths retains byte widths, trailing cell spaces and
// row endl flushes. Unsigned subtraction narrows to source int32 print_blanks.
// Native Go slice headers are borrowed by value; cells remain live reads.
func PrintDebugMatrixWithWidths(rows [][]string, widths []uint32, out io.Writer) error {
	if out == nil {
		return ErrBounds
	}
	for i := 0; i < len(rows); i++ {
		for j := 0; j < len(rows[i]); j++ {
			if j >= len(widths) {
				return ErrBounds
			}
			if err := debugPrintBlanks(int32(widths[j]-uint32(len(rows[i][j]))), out); err != nil {
				return err
			}
			if j >= len(rows[i]) {
				return ErrBounds
			}
			if err := writeConstraintText(out, rows[i][j]); err != nil {
				return err
			}
			if err := writeConstraintText(out, " "); err != nil {
				return err
			}
		}
		if err := writeDriverLine(out, ""); err != nil {
			return err
		}
	}
	return nil
}
func PrintDebugStringMatrix(rows [][]string, out io.Writer) error {
	var widths []uint32
	if len(rows) > 0 {
		for j := 0; j < len(rows[0]); j++ {
			width, err := DebugMatrixColumnWidth(uint32(j), rows)
			if err != nil {
				return err
			}
			widths = append(widths, width)
		}
	}
	if err := PrintDebugMatrixWithWidths(rows, widths, out); err != nil {
		return err
	}
	return writeDriverLine(out, "")
}
func floatDebugMatrixStrings(m FloatDebugMatrix, rows, columns uint32) ([][]string, error) {
	if m == nil {
		return nil, ErrBounds
	}
	table := make([][]string, rows)
	for i := uint32(0); i < rows; i++ {
		for j := uint32(0); j < columns; j++ {
			value, err := m.Element(i, j)
			if err != nil {
				return table, err
			}
			table[i] = append(table[i], floatLPNumberText(value))
		}
	}
	return table, nil
}

// PrintFloatDebugMatrix is the base-matrix pointer overload, including its
// final blank line. All numeric cells are formatted before output begins.
func PrintFloatDebugMatrix(m FloatDebugMatrix, out io.Writer) error {
	if m == nil {
		return ErrBounds
	}
	table, err := floatDebugMatrixStrings(m, m.RowCount(), m.ColumnCount())
	if err != nil {
		return err
	}
	return PrintDebugStringMatrix(table, out)
}
func printFloatDebugMatrixRows(m FloatDebugMatrix, rows, columns uint32, out io.Writer) error {
	table, err := floatDebugMatrixStrings(m, rows, columns)
	if err != nil {
		return err
	}
	widths := make([]uint32, 0, columns)
	for j := uint32(0); j < columns; j++ {
		width, err := DebugMatrixColumnWidth(j, table)
		if err != nil {
			return err
		}
		widths = append(widths, width)
	}
	return PrintDebugMatrixWithWidths(table, widths, out)
}

// The concrete LU overloads omit the base overload's final blank line.
func PrintFloatStaticDebugMatrix(m *FloatStaticMatrix, out io.Writer) error {
	if m == nil {
		return ErrBounds
	}
	return printFloatDebugMatrixRows(m, m.RowCount(), m.ColumnCount(), out)
}
func PrintFloatSparseDebugMatrix(m *FloatSparseMatrix, out io.Writer) error {
	if m == nil {
		return ErrBounds
	}
	return printFloatDebugMatrixRows(m, m.RowCount(), m.ColumnCount(), out)
}
