package numeral

// Source: src/util/mpff.cpp:1117-1120.

import (
	"io"
	"strconv"
)

func (m *FastFloatManager) traceToRational(a *FastFloat, exponent int32) {
	tags := m.mulTraceTags()
	_ = tags.EmitConditional("mpff_to_mpq", "to_mpq_core", "src/util/mpff.cpp", 1119,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeFastFloatPart(w, "to_mpq: "); err != nil {
				return err
			}
			if err := m.DisplayTo(w, a); err != nil {
				return err
			}
			if err := writeFastFloatPart(w, "\nexp: "); err != nil {
				return err
			}
			if err := writeFastFloatPart(w, strconv.FormatInt(int64(exponent), 10)); err != nil {
				return err
			}
			return writeFastFloatPart(w, "\n")
		})
}
