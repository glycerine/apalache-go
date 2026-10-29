package numeral

// Source: src/util/mpf.cpp:1435,1441,1483,1621-1633.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// On the retained Linux AMD64 profile, to_string_hexfloat(mpf) reads the
// first unsigned long long of the C++ object. Its low word is the 15/16/1
// format bitfield and its high word is mpz::m_val. Reconstruct those source
// words rather than reading the unrelated Go object layout.
func (m *ArbitraryFloatManager) writeFloatObjectTrace(w io.Writer, value *ArbitraryFloat, formatError *error) error {
	words, e := m.ValueWords(value)
	if e != nil {
		*formatError = e
		return e
	}
	return writeNumeralDisplayPart(w, fmt.Sprintf("%X", uint64(words[3])<<32|uint64(words[0])))
}

func (m *ArbitraryFloatManager) tracePartialRemainderObject(value *ArbitraryFloat) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_rem") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg_rem", "partial_remainder", "src/util/mpf.cpp", 1435,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "partial remainder = "); e != nil {
				return e
			}
			if e := m.writeFloatObjectTrace(w, value, &formatError); e != nil {
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceRemObjects(x, y *ArbitraryFloat) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_rem") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg_rem", "rem", "src/util/mpf.cpp", 1441,
		func() bool { return true }, func(w io.Writer) error {
			for _, item := range [...]struct {
				label string
				value *ArbitraryFloat
			}{{"X = ", x}, {"Y = ", y}} {
				if e := writeNumeralDisplayPart(w, item.label); e != nil {
					return e
				}
				s, e := m.formatFloatTraceValue(item.value)
				if e != nil {
					formatError = e
					return e
				}
				if e := writeNumeralDisplayPart(w, s+"="); e != nil {
					return e
				}
				if e := m.writeFloatObjectTrace(w, item.value, &formatError); e != nil {
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

func (m *ArbitraryFloatManager) traceRemObjectResult(value *ArbitraryFloat) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_rem") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg_rem", "rem", "src/util/mpf.cpp", 1483,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "R = "); e != nil {
				return e
			}
			s, e := m.formatFloatTraceValue(value)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, s+"="); e != nil {
				return e
			}
			if e := m.writeFloatObjectTrace(w, value, &formatError); e != nil {
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}
