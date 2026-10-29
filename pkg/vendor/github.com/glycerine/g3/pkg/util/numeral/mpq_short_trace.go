package numeral

// Source: src/util/mpq.h:77-154,273-344,458-500.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// BindTrace borrows the inherited integer/rational trace stream. Nil restores
// the process tags for both mpq and nested mpz source operations.
func (m *RationalManager) BindTrace(tags *diagnostic.TraceTags) error {
	if err := m.checkOwnerReady(); err != nil {
		return err
	}
	return m.Z.BindTrace(tags)
}

func (m *RationalManager) traceMPQBinary(a, b, out *Rational, op, tag string, before bool) {
	tags := m.Z.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace(tag) {
		return
	}
	_ = tags.EmitShort(tag, func(w io.Writer) error {
		writeValue := func(v *Rational) error {
			s, err := m.String(v)
			if err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, s)
		}
		if !before {
			if err := writeValue(out); err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, "\n")
		}
		if err := writeNumeralDisplayPart(w, "[mpq] "); err != nil {
			return err
		}
		if err := writeValue(a); err != nil {
			return err
		}
		if err := writeNumeralDisplayPart(w, " "+op+" "); err != nil {
			return err
		}
		if err := writeValue(b); err != nil {
			return err
		}
		return writeNumeralDisplayPart(w, " == ")
	})
}

// traceMPQMixed retains the source operand order without making a shallow
// rational copy of an owned integer numeral.
func (m *RationalManager) traceMPQMixed(q *Rational, z *Int, out *Rational, op, tag string, integerFirst, before bool) {
	tags := m.Z.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace(tag) {
		return
	}
	_ = tags.EmitShort(tag, func(w io.Writer) error {
		writeRational := func(v *Rational) error {
			s, err := m.String(v)
			if err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, s)
		}
		writeInteger := func(v *Int) error {
			s, err := m.Z.String(v)
			if err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, s)
		}
		if !before {
			if err := writeRational(out); err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, "\n")
		}
		if err := writeNumeralDisplayPart(w, "[mpq] "); err != nil {
			return err
		}
		if integerFirst {
			if err := writeInteger(z); err != nil {
				return err
			}
		} else if err := writeRational(q); err != nil {
			return err
		}
		if err := writeNumeralDisplayPart(w, " "+op+" "); err != nil {
			return err
		}
		if integerFirst {
			if err := writeRational(q); err != nil {
				return err
			}
		} else if err := writeInteger(z); err != nil {
			return err
		}
		return writeNumeralDisplayPart(w, " == ")
	})
}
