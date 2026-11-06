package realclosure

// Source: src/math/realclosure/mpz_matrix.cpp:389-406 and
// src/math/realclosure/mpz_matrix.h:101-112.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

func writeSignMatrixText(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err != nil {
		return err
	}
	if n != len(text) {
		return io.ErrShortWrite
	}
	return nil
}

// display preserves mpz_matrix_manager::display's stream insertion order.
// Padding is emitted one space at a time, as in the source loop, so a stopped
// writer exposes the same completed prefix.
func (m *signMatrix) display(w io.Writer, cellWidth uint32) error {
	if m == nil || w == nil || m.rows < 0 || m.cols < 0 || len(m.cells) != m.rows*m.cols {
		return arena.ErrInvalidSlot
	}
	for _, text := range []string{strconv.Itoa(m.rows), " x ", strconv.Itoa(m.cols), " Matrix\n"} {
		if err := writeSignMatrixText(w, text); err != nil {
			return err
		}
	}
	for row := 0; row < m.rows; row++ {
		for col := 0; col < m.cols; col++ {
			if col > 0 {
				if err := writeSignMatrixText(w, " "); err != nil {
					return err
				}
			}
			text := m.at(row, col).String()
			for pad := uint32(len(text)); pad < cellWidth; pad++ {
				if err := writeSignMatrixText(w, " "); err != nil {
					return err
				}
			}
			if err := writeSignMatrixText(w, text); err != nil {
				return err
			}
		}
		if err := writeSignMatrixText(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

// write is the Go counterpart of operator<<(scoped_mpz_matrix), whose source
// route selects display's default cell width.
func (m *signMatrix) write(w io.Writer) error { return m.display(w, 4) }
