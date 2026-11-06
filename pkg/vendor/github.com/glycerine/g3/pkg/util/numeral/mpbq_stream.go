package numeral

// Source: src/util/mpbq.cpp:394-410; retained realclosure endpoint output.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

func writeBinaryRationalPart(w io.Writer, s string) error {
	n, err := io.WriteString(w, s)
	if err == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return err
}

// DisplayTo writes the source's numerator, denominator and exponent insertions
// in order. DisplayPPTo uses the source HTML-style exponent markup.
func (m *BinaryRationalManager) DisplayTo(w io.Writer, a *BinaryRational) error {
	return m.displayTo(w, a, false)
}

func (m *BinaryRationalManager) DisplayPPTo(w io.Writer, a *BinaryRational) error {
	return m.displayTo(w, a, true)
}

func (m *BinaryRationalManager) displayTo(w io.Writer, a *BinaryRational, pretty bool) error {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if err := m.check(a); err != nil {
		return err
	}
	numerator, err := m.Z.String(&a.Num)
	if err != nil {
		return err
	}
	if err := writeBinaryRationalPart(w, numerator); err != nil {
		return err
	}
	if a.KField == 0 {
		return nil
	}
	if err := writeBinaryRationalPart(w, "/2"); err != nil {
		return err
	}
	if a.KField == 1 {
		return nil
	}
	if pretty {
		if err := writeBinaryRationalPart(w, "<sup>"); err != nil {
			return err
		}
	} else if err := writeBinaryRationalPart(w, "^"); err != nil {
		return err
	}
	if err := writeBinaryRationalPart(w, strconv.FormatUint(uint64(a.KField), 10)); err != nil {
		return err
	}
	if pretty {
		return writeBinaryRationalPart(w, "</sup>")
	}
	return nil
}
