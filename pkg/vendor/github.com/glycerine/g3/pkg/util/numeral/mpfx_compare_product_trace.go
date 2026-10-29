package numeral

// Source: src/util/mpfx.cpp:332,351,420,443,450,517.

import "io"

func (m *FixedPointManager) traceFixedLess(a, b *FixedPoint, after, result bool) {
	tags := m.fixedTraceTags()
	if !tags.ShouldTrace("mpfx_trace") {
		return
	}
	_ = tags.EmitShort("mpfx_trace", func(w io.Writer) error {
		if after {
			value := "(0 == 1)\n"
			if result {
				value = "(1 == 1)\n"
			}
			return writeFixedPointPart(w, value)
		}
		if err := writeFixedPointPart(w, "[mpfx] ("); err != nil {
			return err
		}
		if err := m.Display(w, a); err != nil {
			return err
		}
		if err := writeFixedPointPart(w, " < "); err != nil {
			return err
		}
		if err := m.Display(w, b); err != nil {
			return err
		}
		return writeFixedPointPart(w, ") == ")
	})
}

func (m *FixedPointManager) traceFixedProduct(a, b, out *FixedPoint, divide, after bool) {
	tags := m.fixedTraceTags()
	if !tags.ShouldTrace("mpfx_trace") {
		return
	}
	_ = tags.EmitShort("mpfx_trace", func(w io.Writer) error {
		if after {
			if err := m.Display(w, out); err != nil {
				return err
			}
			return writeFixedPointPart(w, "\n")
		}
		if err := writeFixedPointPart(w, "[mpfx] ("); err != nil {
			return err
		}
		if err := m.Display(w, a); err != nil {
			return err
		}
		op := ") * ("
		if divide {
			op = ") / ("
		}
		if err := writeFixedPointPart(w, op); err != nil {
			return err
		}
		if err := m.Display(w, b); err != nil {
			return err
		}
		bound := ") >= "
		if m.roundUp {
			bound = ") <= "
		}
		return writeFixedPointPart(w, bound)
	})
}
