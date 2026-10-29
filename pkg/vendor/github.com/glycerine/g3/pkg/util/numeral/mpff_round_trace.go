package numeral

// Source: src/util/mpff.cpp:918-987.

import "io"

func (m *FastFloatManager) traceRoundInteger(a *FastFloat, ceil, before bool) {
	tags := m.mulTraceTags()
	if !tags.ShouldTrace("mpff_trace") {
		return
	}
	_ = tags.EmitShort("mpff_trace", func(w io.Writer) error {
		if !before {
			if err := m.DisplayTo(w, a); err != nil {
				return err
			}
			return writeFastFloatPart(w, "\n")
		}
		name := "[mpff] Floor["
		if ceil {
			name = "[mpff] Ceiling["
		}
		if err := writeFastFloatPart(w, name); err != nil {
			return err
		}
		if err := m.DisplayTo(w, a); err != nil {
			return err
		}
		return writeFastFloatPart(w, "] == ")
	})
}
