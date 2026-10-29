package numeral

// Source: src/util/mpff.cpp:420-469.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// BindTrace borrows the source tag stream for this concrete MPFF manager.
// Nil restores the process-global tags.
func (m *FastFloatManager) BindTrace(tags *diagnostic.TraceTags) error {
	if err := m.open(); err != nil {
		return err
	}
	m.trace = tags
	return nil
}

func (m *FastFloatManager) traceLessOperands(a, b *FastFloat) {
	tags := m.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("mpff_trace") {
		return
	}
	_ = tags.EmitShort("mpff_trace", func(w io.Writer) error {
		if err := writeFastFloatPart(w, "[mpff] ("); err != nil {
			return err
		}
		if err := m.DisplayTo(w, a); err != nil {
			return err
		}
		if err := writeFastFloatPart(w, " < "); err != nil {
			return err
		}
		if err := m.DisplayTo(w, b); err != nil {
			return err
		}
		return writeFastFloatPart(w, ") == ")
	})
}

func (m *FastFloatManager) traceLessResult(result bool, computed bool) {
	tags := m.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("mpff_trace") {
		return
	}
	body := "(1 == 0)\n"
	if computed {
		body = "(0 == 1)\n"
	}
	if result {
		body = "(1 == 1)\n"
	}
	_ = tags.EmitShort("mpff_trace", func(w io.Writer) error {
		if !computed {
			return writeFastFloatPart(w, body)
		}
		if err := writeFastFloatPart(w, "("); err != nil {
			return err
		}
		if err := writeFastFloatPart(w, traceBoolDigit(result)); err != nil {
			return err
		}
		return writeFastFloatPart(w, " == 1)\n")
	})
}
