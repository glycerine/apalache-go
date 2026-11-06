package numeral

// Source basic_interval display.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
)

func writeBinaryIntervalPart(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}

// Display follows basic_interval_manager<mpbq_manager,false>::display:
// each delimiter and completed bound string is inserted in source order.
func (m *BinaryIntervalManager) Display(w io.Writer, a *BinaryInterval) error {
	return m.DisplayTracked(w, a, nil, nil)
}

func (m *BinaryIntervalManager) DisplayTracked(w io.Writer, a *BinaryInterval, lowerReceipt, upperReceipt *NumeralInsertionReceipt) error {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if err := m.check(a); err != nil {
		return err
	}
	if err := writeBinaryIntervalPart(w, "("); err != nil {
		return err
	}
	lower, err := m.bounds.SourceStringTracked(&a.lower, lowerReceipt)
	if err != nil {
		return err
	}
	if err := writeBinaryIntervalPart(w, lower); err != nil {
		return err
	}
	if err := writeBinaryIntervalPart(w, ", "); err != nil {
		return err
	}
	upper, err := m.bounds.SourceStringTracked(&a.upper, upperReceipt)
	if err != nil {
		return err
	}
	if err := writeBinaryIntervalPart(w, upper); err != nil {
		return err
	}
	return writeBinaryIntervalPart(w, ")")
}

func (m *BinaryIntervalManager) appendBound(a *BinaryRational, offset uint32) (uint32, error) {
	n, e := m.bounds.Z.prepareInteger(&a.Num, offset)
	if e != nil || a.KField == 0 {
		return n, e
	}
	n, e = m.bounds.Z.Natural.appendDisplay(n, "/2")
	if e != nil || a.KField == 1 {
		return n, e
	}
	n, e = m.bounds.Z.Natural.appendDisplay(n, "^")
	if e != nil {
		return n, e
	}
	return m.bounds.appendUnsigned(n, a.KField)
}
func (m *BinaryIntervalManager) prepareInterval(a *BinaryInterval) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	w := m.bounds.Z.Natural
	n, e := w.appendDisplay(0, "(")
	if e != nil {
		return 0, e
	}
	n, e = m.appendBound(&a.lower, n)
	if e != nil {
		return 0, e
	}
	n, e = w.appendDisplay(n, ", ")
	if e != nil {
		return 0, e
	}
	n, e = m.appendBound(&a.upper, n)
	if e != nil {
		return 0, e
	}
	return w.appendDisplay(n, ")")
}
func (m *BinaryIntervalManager) String(a *BinaryInterval) (string, error) {
	n, e := m.prepareInterval(a)
	if e != nil {
		return "", e
	}
	return m.bounds.Z.Natural.finalString(n)
}
func (m *BinaryIntervalManager) FormatInto(a *BinaryInterval, dst []byte) (int, error) {
	n, e := m.prepareInterval(a)
	if e != nil {
		return 0, e
	}
	return m.bounds.Z.Natural.publishText(dst, n)
}
