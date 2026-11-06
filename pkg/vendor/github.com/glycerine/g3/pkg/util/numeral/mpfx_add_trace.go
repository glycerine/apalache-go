package numeral

// Source: src/util/mpfx.cpp:368,403,408-416.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *FixedPointManager) BindTrace(tags *diagnostic.TraceTags) error {
	if m == nil || m.self != m {
		return ErrFixedPointOwner
	}
	if m.closed {
		return ErrFixedPointClosed
	}
	m.trace = tags
	return nil
}

func (m *FixedPointManager) fixedTraceTags() *diagnostic.TraceTags {
	if m.trace != nil {
		return m.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (m *FixedPointManager) traceFixedAddSubOuter(a, b, out *FixedPoint, sub, before bool) {
	tags := m.fixedTraceTags()
	if !tags.ShouldTrace("mpfx_trace") {
		return
	}
	_ = tags.EmitShort("mpfx_trace", func(w io.Writer) error {
		if !before {
			if err := m.Display(w, out); err != nil {
				return err
			}
			return writeFixedPointPart(w, "\n")
		}
		if err := writeFixedPointPart(w, "[mpfx] "); err != nil {
			return err
		}
		if err := m.Display(w, a); err != nil {
			return err
		}
		op := " + "
		if sub {
			op = " - "
		}
		if err := writeFixedPointPart(w, op); err != nil {
			return err
		}
		if err := m.Display(w, b); err != nil {
			return err
		}
		return writeFixedPointPart(w, " == ")
	})
}

func (m *FixedPointManager) traceFixedAddSubInner(a, b, out *FixedPoint, sub, before bool) {
	tags := m.fixedTraceTags()
	if !tags.ShouldTrace("mpfx") {
		return
	}
	line := 403
	if before {
		line = 368
	}
	_ = tags.EmitConditional("mpfx", "add_sub", "src/util/mpfx.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if !before {
				if err := writeFixedPointPart(w, "result: "); err != nil {
					return err
				}
				if err := m.Display(w, out); err != nil {
					return err
				}
				return writeFixedPointPart(w, "\n")
			}
			name := "add("
			if sub {
				name = "sub("
			}
			if err := writeFixedPointPart(w, name); err != nil {
				return err
			}
			if err := m.Display(w, a); err != nil {
				return err
			}
			if err := writeFixedPointPart(w, ", "); err != nil {
				return err
			}
			if err := m.Display(w, b); err != nil {
				return err
			}
			return writeFixedPointPart(w, ")\n")
		})
}
