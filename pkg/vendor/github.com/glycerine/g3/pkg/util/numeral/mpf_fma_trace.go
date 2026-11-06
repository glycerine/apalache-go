package numeral

// Source: src/util/mpf.cpp:813-818,830-831,844-845,867-874,899-940.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ArbitraryFloatManager) traceFloatBinaryString(value *ArbitraryFloat, upper, lower uint32) (string, error) {
	var out utext.OwnedString
	previousPending := m.binaryString.active
	if e := m.ToStringBinary(value, upper, lower, &out); e != nil {
		var cleanup error
		if !previousPending {
			cleanup = m.CleanupBinaryString()
		}
		out.OwnAbort()
		return "", JoinNumericErrors(e, cleanup)
	}
	defer out.SourceDestroy()
	return out.View().String()
}

func (m *ArbitraryFloatManager) traceFMAUnpacked(a, b, c *ArbitraryFloat) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	values := [...]*ArbitraryFloat{a, b, c}
	labels := [...]string{"A = ", "B = ", "C = "}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "fma", "src/util/mpf.cpp", 813,
		func() bool { return true }, func(w io.Writer) error {
			for i, value := range values {
				if e := writeNumeralDisplayPart(w, labels[i]); e != nil {
					return e
				}
				formatted, e := m.formatFloatTraceValue(value)
				if e != nil {
					formatError = e
					return e
				}
				if e := writeNumeralDisplayPart(w, formatted); e != nil {
					return e
				}
				if e := diagnostic.WriteTraceEndl(w); e != nil {
					return e
				}
			}
			for i, value := range values {
				if e := writeNumeralDisplayPart(w, labels[i]); e != nil {
					return e
				}
				binary, e := m.traceFloatBinaryString(value, 1, 0)
				if e != nil {
					formatError = e
					return e
				}
				if e := writeNumeralDisplayPart(w, binary); e != nil {
					return e
				}
				if e := diagnostic.WriteTraceEndl(w); e != nil {
					return e
				}
			}
			return nil
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceFMADecimalBinary(label string, value *ArbitraryFloat, line int) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "fma", "src/util/mpf.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, label); e != nil {
				return e
			}
			decimal, e := m.formatFloatTraceValue(value)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, decimal); e != nil {
				return e
			}
			if e := diagnostic.WriteTraceEndl(w); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, label); e != nil {
				return e
			}
			binary, e := m.traceFloatBinaryString(value, 1, 0)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, binary); e != nil {
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceFMAAlignment(delta uint32, shifted, sticky *Int) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "fma", "src/util/mpf.cpp", 867,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "alignment shift by delta="); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, strconv.FormatUint(uint64(delta), 10)); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, " -> sig = "); e != nil {
				return e
			}
			s, e := m.integer.String(shifted)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, s); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, " sticky_rem = "); e != nil {
				return e
			}
			s, e = m.integer.String(sticky)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, s); e != nil {
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceFMASignificandBinary(decimalLabel, binaryLabel string, value *ArbitraryFloat, upper, lower uint32, line int) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "fma", "src/util/mpf.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, decimalLabel); e != nil {
				return e
			}
			s, e := m.integer.String(&value.significand)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, s); e != nil {
				return e
			}
			if e := diagnostic.WriteTraceEndl(w); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, binaryLabel); e != nil {
				return e
			}
			binary, e := m.traceFloatBinaryString(value, upper, lower)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, binary); e != nil {
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceFMAOverflow(value *ArbitraryFloat) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "fma", "src/util/mpf.cpp", 913,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "Add/Sub overflew into 4.xxx!"); e != nil {
				return e
			}
			if e := diagnostic.WriteTraceEndl(w); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, "S*= "); e != nil {
				return e
			}
			binary, e := m.traceFloatBinaryString(value, 2, 0)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, binary); e != nil {
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceFMARenormalized(value *ArbitraryFloat, delta uint32) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "fma", "src/util/mpf.cpp", 925,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "R*= "); e != nil {
				return e
			}
			binary, e := m.traceFloatBinaryString(value, 2, 0)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, binary); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, " (renormalized, delta="); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, strconv.FormatUint(uint64(delta), 10)); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, ")"); e != nil {
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}
