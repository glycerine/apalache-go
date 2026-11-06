package numeral

// Source: src/util/mpbq.cpp:426-458.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
)

// DisplayDecimalTo emits the source's sign, integer part, decimal point,
// individual digits and uncertainty marker in order. A failed insertion
// leaves the raw numeric display locals for explicit CleanupDisplay.
func (m *BinaryRationalManager) DisplayDecimalTo(w io.Writer, a *BinaryRational, prec uint32) (err error) {
	return m.DisplayDecimalToWithInvocation(w, a, prec, nil)
}

func (m *BinaryRationalManager) DisplayDecimalToWithInvocation(w io.Writer, a *BinaryRational, prec uint32, receipt *BinaryDisplayInvocationReceipt) (err error) {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if err = m.check(a); err != nil {
		return err
	}
	if a.KField == 0 {
		text, formatErr := m.Z.String(&a.Num)
		if formatErr != nil {
			return formatErr
		}
		return writeBinaryRationalPart(w, text)
	}
	if err = m.beginBinaryDisplay(); err != nil {
		return err
	}
	if receipt != nil {
		*receipt = BinaryDisplayInvocationReceipt{m, m.Display.invocation}
	}
	defer func() {
		var unused uint32
		m.finishBinaryDisplay(&unused, &err)
	}()
	two, ten := NewInt(2), NewInt(10)
	n1, v1, twok := &m.Display.Values[0], &m.Display.Values[1], &m.Display.Values[2]
	if a.Num.Val < 0 {
		if err = writeBinaryRationalPart(w, "-"); err != nil {
			return err
		}
	}
	if err = m.Z.Copy(v1, &a.Num); err != nil {
		return err
	}
	if err = m.Z.Abs(v1); err != nil {
		return err
	}
	if err = m.Z.Power(&two, a.KField, twok); err != nil {
		return err
	}
	if err = m.Z.Rem(v1, twok, n1); err != nil {
		return err
	}
	if err = m.Z.Div(v1, twok, v1); err != nil {
		return err
	}
	if err = m.debugAssert(n1.Val != 0); err != nil {
		return err
	}
	text, err := m.Z.String(v1)
	if err != nil {
		return err
	}
	if err = writeBinaryRationalPart(w, text); err != nil {
		return err
	}
	if err = writeBinaryRationalPart(w, "."); err != nil {
		return err
	}
	for i := uint32(0); i < prec; i++ {
		if err = m.Z.Mul(n1, &ten, n1); err != nil {
			return err
		}
		if err = m.Z.Div(n1, twok, v1); err != nil {
			return err
		}
		if err = m.Z.Rem(n1, twok, n1); err != nil {
			return err
		}
		text, err = m.Z.String(v1)
		if err != nil {
			return err
		}
		if err = writeBinaryRationalPart(w, text); err != nil {
			return err
		}
		if n1.Val == 0 {
			return nil
		}
	}
	return writeBinaryRationalPart(w, "?")
}
