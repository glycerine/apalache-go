package numeral

// Source: src/util/mpff.cpp:328-372.

import (
	"io"
	"strconv"
)

func (m *FastFloatManager) traceSetInteger(line int, body func(io.Writer) error) {
	tags := m.mulTraceTags()
	_ = tags.EmitConditional("mpff", "set_core", "src/util/mpff.cpp", line,
		func() bool { return true }, body)
}

func (m *FastFloatManager) traceSetIntegerInput(z *IntegerManager, v *Int) {
	m.traceSetInteger(329, func(w io.Writer) error {
		if err := writeFastFloatPart(w, "mpz->mpff\n"); err != nil {
			return err
		}
		if err := z.DisplayTo(w, v); err != nil {
			return err
		}
		return writeFastFloatPart(w, "\n")
	})
}

func (m *FastFloatManager) traceSetIntegerSigned(v int64) {
	m.traceSetInteger(331, func(w io.Writer) error {
		if err := writeFastFloatPart(w, "is_int64 "); err != nil {
			return err
		}
		if err := writeFastFloatPart(w, strconv.FormatInt(v, 10)); err != nil {
			return err
		}
		return writeFastFloatPart(w, "\n")
	})
}

func (m *FastFloatManager) traceSetIntegerUnsigned() {
	m.traceSetInteger(335, func(w io.Writer) error {
		return writeFastFloatPart(w, "is_uint64\n")
	})
}

func (m *FastFloatManager) traceSetIntegerWords(w ffLimbView) {
	m.traceSetInteger(345, func(out io.Writer) error {
		if err := writeFastFloatPart(out, "w words: "); err != nil {
			return err
		}
		for i := uint32(0); i < w.size; i++ {
			if err := writeFastFloatPart(out, strconv.FormatUint(uint64(w.read(i)), 10)); err != nil {
				return err
			}
			if err := writeFastFloatPart(out, " "); err != nil {
				return err
			}
		}
		return writeFastFloatPart(out, "\n")
	})
}

func (m *FastFloatManager) traceSetIntegerResult(a *FastFloat) {
	m.traceSetInteger(371, func(w io.Writer) error {
		if err := writeFastFloatPart(w, "mpz->mpff result:\n"); err != nil {
			return err
		}
		if err := m.displayRawTraceTo(w, a); err != nil {
			return err
		}
		return writeFastFloatPart(w, "\n")
	})
}
