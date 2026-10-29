package numeral

// Source: src/util/mpfx.h:267-268; src/util/mpfx.cpp:221-234,447-517.

import (
	"io"
	"strconv"
)

func (m *FixedPointManager) traceFixedIntegerDivision(num int64, den uint64, out *FixedPoint, after bool) {
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
		bound := ">="
		if m.roundUp {
			bound = "<="
		}
		if err := writeFixedPointPart(w, "[mpfx] ("); err != nil {
			return err
		}
		digits := strconv.FormatInt(num, 10)
		if num < 0 {
			if err := writeFixedPointPart(w, "-"); err != nil {
				return err
			}
			digits = digits[1:]
		}
		for _, part := range [...]string{digits, ") / (", strconv.FormatUint(den, 10), ") ", bound, " "} {
			if err := writeFixedPointPart(w, part); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *FixedPointManager) traceFixedInverse(a *FixedPoint, after bool) {
	tags := m.fixedTraceTags()
	if !tags.ShouldTrace("mpfx_trace") {
		return
	}
	_ = tags.EmitShort("mpfx_trace", func(w io.Writer) error {
		if after {
			if err := m.Display(w, a); err != nil {
				return err
			}
			return writeFixedPointPart(w, "\n")
		}
		if err := writeFixedPointPart(w, "[mpfx] (1) / ("); err != nil {
			return err
		}
		if err := m.Display(w, a); err != nil {
			return err
		}
		bound := ">="
		if m.roundUp {
			bound = "<="
		}
		for _, part := range [...]string{") ", bound, " "} {
			if err := writeFixedPointPart(w, part); err != nil {
				return err
			}
		}
		return nil
	})
}
