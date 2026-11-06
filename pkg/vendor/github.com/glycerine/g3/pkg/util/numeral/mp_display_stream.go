package numeral

// Source: src/util/mpz.cpp:1385-1408 and src/util/mpq.cpp:130-139.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

type numeralDisplayStream struct{ io.Writer }

func (out numeralDisplayStream) AppendInt32(v int32) error {
	if w, ok := out.Writer.(interface{ AppendInt32(int32) error }); ok {
		return w.AppendInt32(v)
	}
	return writeNumeralDisplayPart(out.Writer, strconv.FormatInt(int64(v), 10))
}
func (out numeralDisplayStream) AppendCString(s string) error {
	return writeNumeralDisplayPart(out.Writer, s)
}
func (out numeralDisplayStream) AppendNumeralText(s string) error {
	return writeNumeralDisplayPart(out.Writer, s)
}

func writeNumeralDisplayPart(w io.Writer, s string) error {
	// A receiving numeric stream owns its formatting state. Keep CString
	// insertion distinct from raw io.Writer bytes (notably width/fill).
	if out, ok := w.(interface{ AppendCString(string) error }); ok {
		return out.AppendCString(s)
	}
	n, err := io.WriteString(w, s)
	if err == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return err
}

// DisplayTo writes an inline integer in one insertion. A large negative
// integer writes its sign before formatting and inserting the magnitude.
// Receiving numeric/string insertion methods retain their actual stream flags.
func (m *IntegerManager) DisplayTo(w io.Writer, a *Int) error {
	return m.DisplayToTracked(w, a, nil)
}

// DisplayToTracked exposes the exact BIG character/formatting child to a
// retained source consumer. SMALL values remain direct numeric insertions.
func (m *IntegerManager) DisplayToTracked(w io.Writer, a *Int, receipt *NumeralInsertionReceipt) error {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	return m.InsertSourceIntegerTracked(numeralDisplayStream{w}, a, receipt, nil)
}

// DisplayTo preserves the numerator, slash and denominator source insertions.
func (m *RationalManager) DisplayTo(w io.Writer, a *Rational) error {
	return m.DisplayToTracked(w, a, nil)
}

// DisplayToTracked reuses one exact receipt in source order: a completed
// numerator owns no child before a denominator insertion can begin.
func (m *RationalManager) DisplayToTracked(w io.Writer, a *Rational, receipt *NumeralInsertionReceipt) error {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if err := m.check(a); err != nil {
		return err
	}
	integer := isIntegerRational(a)
	if err := m.Z.DisplayToTracked(w, &a.Num, receipt); err != nil {
		return err
	}
	if integer {
		return nil
	}
	if err := writeNumeralDisplayPart(w, "/"); err != nil {
		return err
	}
	return m.Z.DisplayToTracked(w, &a.Den, receipt)
}

// CleanupSourceDisplay acknowledges only the insertion issued to this owner.
func (m *IntegerManager) CleanupSourceDisplay(receipt *NumeralInsertionReceipt) error {
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	if receipt == nil || !receipt.Active() {
		return nil
	}
	return cleanupOwnedNumeralInsertion(receipt, m.Naturals())
}
