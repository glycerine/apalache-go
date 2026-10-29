package numeral

// Source: src/util/mpff.cpp:989-1054.

import (
	"fmt"
	"io"
	"strconv"
)

func (m *FastFloatManager) tracePower(a *FastFloat, p uint32, b *FastFloat) {
	tags := m.mulTraceTags()
	_ = tags.EmitShort("mpff_trace", func(w io.Writer) error {
		if err := writeFastFloatPart(w, "[mpff] ("); err != nil {
			return err
		}
		if err := m.DisplayTo(w, a); err != nil {
			return err
		}
		if err := writeFastFloatPart(w, ") ^ "); err != nil {
			return err
		}
		if err := writeFastFloatPart(w, strconv.FormatUint(uint64(p), 10)); err != nil {
			return err
		}
		bound := ">="
		if m.toPlus {
			bound = "<="
		}
		if err := writeFastFloatPart(w, bound); err != nil {
			return err
		}
		if err := m.DisplayTo(w, b); err != nil {
			return err
		}
		return writeFastFloatPart(w, "\n")
	})
	_ = tags.EmitConditional("mpff_power", "power", "src/util/mpff.cpp", 1053,
		func() bool { return true }, func(w io.Writer) error {
			if err := m.displayRawTraceTo(w, b); err != nil {
				return err
			}
			return writeFastFloatPart(w, "\n")
		})
}

// displayRawTraceTo follows display_raw's sign, high-to-low hex word, and
// exponent insertions so a short write leaves the same completed prefix.
func (m *FastFloatManager) displayRawTraceTo(w io.Writer, b *FastFloat) error {
	if b.negative() {
		if err := writeFastFloatPart(w, "-"); err != nil {
			return err
		}
	}
	s := m.sig(b)
	for i := s.size; i > 0; {
		i--
		if err := writeFastFloatPart(w, fmt.Sprintf("%08x", s.read(i))); err != nil {
			return err
		}
	}
	if err := writeFastFloatPart(w, "*2^"); err != nil {
		return err
	}
	return writeFastFloatPart(w, strconv.FormatInt(int64(b.exponent), 10))
}
