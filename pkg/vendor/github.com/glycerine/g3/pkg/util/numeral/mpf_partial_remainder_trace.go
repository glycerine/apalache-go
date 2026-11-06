package numeral

// Source: src/util/mpf.cpp:1282-1285,1322,1365,1394,1398,1421.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func writePartialRemainderScalarLine(w io.Writer, label, value string) error {
	if e := writeNumeralDisplayPart(w, label); e != nil {
		return e
	}
	if e := writeNumeralDisplayPart(w, value); e != nil {
		return e
	}
	return diagnostic.WriteTraceEndl(w)
}

func (m *ArbitraryFloatManager) tracePartialRemainderEntry(x, y *ArbitraryFloat, d int64, partial bool) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_rem") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg_rem", "partial_remainder", "src/util/mpf.cpp", 1282,
		func() bool { return true }, func(w io.Writer) error {
			for _, item := range [...]struct {
				label string
				value *ArbitraryFloat
			}{{"x=", x}, {"y=", y}} {
				if e := writeNumeralDisplayPart(w, item.label); e != nil {
					return e
				}
				s, e := m.formatFloatTraceValue(item.value)
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
			}
			if e := writePartialRemainderScalarLine(w, "d=", strconv.FormatInt(d, 10)); e != nil {
				return e
			}
			return writePartialRemainderScalarLine(w, "partial=", traceBoolDigit(partial))
		})
	return formatError
}

func (m *ArbitraryFloatManager) tracePartialRemainderTie(tie, more bool) {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_rem") {
		return
	}
	_ = tags.EmitConditional("mpf_dbg_rem", "partial_remainder", "src/util/mpf.cpp", 1322,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{"tie= ", traceBoolDigit(tie), "; >tie= ", traceBoolDigit(more)} {
				if e := writeNumeralDisplayPart(w, part); e != nil {
					return e
				}
			}
			return diagnostic.WriteTraceEndl(w)
		})
}

func (m *ArbitraryFloatManager) tracePartialRemainderExponentDelta(delta int64) {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_rem") {
		return
	}
	_ = tags.EmitConditional("mpf_dbg_rem", "partial_remainder", "src/util/mpf.cpp", 1365,
		func() bool { return true }, func(w io.Writer) error {
			return writePartialRemainderScalarLine(w, "exp_delta=", strconv.FormatInt(delta, 10))
		})
}

func (m *ArbitraryFloatManager) tracePartialRemainderInt(line int, label string, value *Int) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_rem") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg_rem", "partial_remainder", "src/util/mpf.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, label); e != nil {
				return e
			}
			s, e := m.integer.String(value)
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
