package numeral

// Source: src/util/mpf.cpp:541-579,590-650,664,670-742,752-754,852,880,892.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ArbitraryFloatManager) BindTrace(tags *diagnostic.TraceTags) error {
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	m.trace = tags
	return nil
}

func (m *ArbitraryFloatManager) floatTraceTags() *diagnostic.TraceTags {
	if m.trace != nil {
		return m.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (m *ArbitraryFloatManager) formatFloatTraceValue(value *ArbitraryFloat) (result string, cause error) {
	text, err := NewArbitraryFloatFormatText(m.budget)
	if err != nil {
		return "", err
	}
	defer func() { cause = JoinNumericErrors(cause, text.Close()) }()
	view, err := m.ToStringInto(value, text)
	if err != nil {
		return "", err
	}
	n, err := view.Length()
	if err != nil {
		return "", err
	}
	if n > uint64(int(^uint(0)>>1)) {
		return "", arena.ErrBounds
	}
	data := make([]byte, int(n))
	for i := range data {
		b, e := view.Byte(uint64(i))
		if e != nil {
			return "", e
		}
		data[i] = b
	}
	return string(data), nil
}

// ToString exposes the concrete MPF manager formatter to retained AST TRACE
// callers while keeping its owned text context and cleanup in one place.
func (m *ArbitraryFloatManager) ToString(value *ArbitraryFloat) (string, error) {
	return m.formatFloatTraceValue(value)
}

func (m *ArbitraryFloatManager) traceFloatString(w io.Writer, value *ArbitraryFloat) error {
	text, e := m.formatFloatTraceValue(value)
	if e != nil {
		return e
	}
	return writeNumeralDisplayPart(w, text)
}

func (m *ArbitraryFloatManager) traceMulEntry(mode ArbitraryFloatRoundingMode, x, y *ArbitraryFloat) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_mul_bug") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_mul_bug", "mul", "src/util/mpf.cpp", 590,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "rm: "); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, strconv.FormatUint(uint64(mode), 10)); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, "\n"); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, "X: "); e != nil {
				return e
			}
			if e := m.traceFloatString(w, x); e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, "\n"); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, "Y: "); e != nil {
				return e
			}
			if e := m.traceFloatString(w, y); e != nil {
				formatError = e
				return e
			}
			return writeNumeralDisplayPart(w, "\n")
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceMulResult(out *ArbitraryFloat) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_mul_bug") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_mul_bug", "mul", "src/util/mpf.cpp", 664,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "result: "); e != nil {
				return e
			}
			if e := m.traceFloatString(w, out); e != nil {
				formatError = e
				return e
			}
			return writeNumeralDisplayPart(w, "\n")
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceFloatDebugValueChecked(function, label string, value *ArbitraryFloat, line int) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", function, "src/util/mpf.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, label); e != nil {
				return e
			}
			if e := m.traceFloatString(w, value); e != nil {
				formatError = e
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceFloatDebugIntChecked(function, label string, value *Int, line int) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", function, "src/util/mpf.cpp", line,
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

func (m *ArbitraryFloatManager) traceFloatDebugText(function, value string, line int) {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return
	}
	_ = tags.EmitConditional("mpf_dbg", function, "src/util/mpf.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, value); e != nil {
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
}

func (m *ArbitraryFloatManager) traceFloatDebugSigned(function, label string, value int64, line int) {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return
	}
	_ = tags.EmitConditional("mpf_dbg", function, "src/util/mpf.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, label); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, strconv.FormatInt(value, 10)); e != nil {
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
}
