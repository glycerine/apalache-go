package numeral

// Source: src/util/mpff.cpp:646,765,769-779.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *FastFloatManager) traceAddSubOuter(a, b, c *FastFloat, sub, before bool) {
	tags := m.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
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
		if err := writeFastFloatPart(w, "[mpff] "); err != nil {
			return err
		}
		if err := m.DisplayTo(w, a); err != nil {
			return err
		}
		op := " + "
		if sub {
			op = " - "
		}
		if err := writeFastFloatPart(w, op); err != nil {
			return err
		}
		if err := m.DisplayTo(w, b); err != nil {
			return err
		}
		if err := writeFastFloatPart(w, " "); err != nil {
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

func (m *FastFloatManager) traceAddSubInner(a, b, c *FastFloat, sub, before bool) {
	tags := m.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("mpff") {
		return
	}
	line := 765
	if before {
		line = 646
	}
	_ = tags.EmitConditional("mpff", "add_sub", "src/util/mpff.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if !before {
				if err := writeFastFloatPart(w, "result: "); err != nil {
					return err
				}
				if err := m.DisplayTo(w, c); err != nil {
					return err
				}
				return writeFastFloatPart(w, "\n")
			}
			name := "add"
			if sub {
				name = "sub"
			}
			if err := writeFastFloatPart(w, name); err != nil {
				return err
			}
			if err := writeFastFloatPart(w, "("); err != nil {
				return err
			}
			if err := m.DisplayTo(w, a); err != nil {
				return err
			}
			if err := writeFastFloatPart(w, ", "); err != nil {
				return err
			}
			if err := m.DisplayTo(w, b); err != nil {
				return err
			}
			return writeFastFloatPart(w, ")\n")
		})
}
