package numeral

// Source: src/util/mpff.cpp:827-914.

import (
	"fmt"
	"io"
)

func (m *FastFloatManager) traceDivOuter(a, b, c *FastFloat, before bool) {
	tags := m.mulTraceTags()
	if !tags.ShouldTrace("mpff_trace") {
		return
	}
	_ = tags.EmitShort("mpff_trace", func(w io.Writer) error {
		if !before {
			if err := m.DisplayTo(w, c); err != nil {
				return err
			}
			return writeFastFloatPart(w, "\n")
		}
		if err := writeFastFloatPart(w, "[mpff] ("); err != nil {
			return err
		}
		if err := m.DisplayTo(w, a); err != nil {
			return err
		}
		if err := writeFastFloatPart(w, ") / ("); err != nil {
			return err
		}
		if err := m.DisplayTo(w, b); err != nil {
			return err
		}
		if err := writeFastFloatPart(w, ") "); err != nil {
			return err
		}
		bound := ">="
		if m.toPlus {
			bound = "<="
		}
		if err := writeFastFloatPart(w, bound); err != nil {
			return err
		}
		return writeFastFloatPart(w, " ")
	})
}

// The source inserts one zero-padded, eight-column lowercase hex word at a
// time, from the quotient's highest limb down to its lowest, then a newline.
func (m *FastFloatManager) traceDivQuotient(q ffLimbView) {
	tags := m.mulTraceTags()
	if !tags.ShouldTrace("mpff_div") {
		return
	}
	_ = tags.EmitConditional("mpff_div", "div", "src/util/mpff.cpp", 871,
		func() bool { return true }, func(w io.Writer) error {
			for i := q.size; i > 0; {
				i--
				if err := writeFastFloatPart(w, fmt.Sprintf("%08x", q.read(i))); err != nil {
					return err
				}
				if err := writeFastFloatPart(w, " "); err != nil {
					return err
				}
			}
			return writeFastFloatPart(w, "\n")
		})
}
