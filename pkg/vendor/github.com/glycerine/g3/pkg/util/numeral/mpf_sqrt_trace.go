package numeral

// Source: src/util/mpf.cpp:968-994.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *IntegerManager) traceMPFSqrtBounds(input, lower, upper *Int) {
	tags := m.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("mpf_dbg") {
		return
	}
	_ = tags.EmitShort("mpf_dbg", func(w io.Writer) error {
		for _, part := range []struct {
			prefix string
			value  *Int
		}{{"SIG = ", input}, {" LOWER = ", lower}, {" UPPER = ", upper}} {
			if err := writeNumeralDisplayPart(w, part.prefix); err != nil {
				return err
			}
			s, err := m.String(part.value)
			if err != nil {
				return err
			}
			if err := writeNumeralDisplayPart(w, s); err != nil {
				return err
			}
		}
		return diagnostic.WriteTraceEndl(w)
	})
}

func (m *IntegerManager) traceMPFSqrtMid(mid, product *Int) {
	tags := m.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("mpf_dbg") {
		return
	}
	_ = tags.EmitShort("mpf_dbg", func(w io.Writer) error {
		if err := writeNumeralDisplayPart(w, "MID = "); err != nil {
			return err
		}
		s, err := m.String(mid)
		if err != nil {
			return err
		}
		if err := writeNumeralDisplayPart(w, s); err != nil {
			return err
		}
		if err := writeNumeralDisplayPart(w, " PROD = "); err != nil {
			return err
		}
		s, err = m.String(product)
		if err != nil {
			return err
		}
		if err := writeNumeralDisplayPart(w, s); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(w)
	})
}

func (m *IntegerManager) traceMPFSqrtChoice(lower bool) {
	tags := m.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("mpf_dbg") {
		return
	}
	choice := "choosing upper"
	if lower {
		choice = "choosing lower"
	}
	_ = tags.EmitShort("mpf_dbg", func(w io.Writer) error {
		if e := writeNumeralDisplayPart(w, choice); e != nil {
			return e
		}
		return diagnostic.WriteTraceEndl(w)
	})
}
