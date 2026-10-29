package numeral

// Source: src/util/mpfx.cpp:559,579,583,603.

import "io"

func (m *FixedPointManager) traceFixedRoundIntegral(n *FixedPoint, ceil, after bool) {
	tags := m.fixedTraceTags()
	if !tags.ShouldTrace("mpfx_trace") {
		return
	}
	_ = tags.EmitShort("mpfx_trace", func(w io.Writer) error {
		if after {
			if err := m.Display(w, n); err != nil {
				return err
			}
			return writeFixedPointPart(w, "\n")
		}
		name := "[mpfx] Floor["
		if ceil {
			name = "[mpfx] Ceiling["
		}
		if err := writeFixedPointPart(w, name); err != nil {
			return err
		}
		if err := m.Display(w, n); err != nil {
			return err
		}
		return writeFixedPointPart(w, "] == ")
	})
}
