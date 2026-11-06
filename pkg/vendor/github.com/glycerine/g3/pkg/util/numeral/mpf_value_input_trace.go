package numeral

// Source: src/util/mpf.cpp:76,105,124-125,147,160-161,183,187,191,195,219,230,235,270,289,300.

import (
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func writeSetTracePart(w io.Writer, part string) error {
	if part == "\n" {
		return diagnostic.WriteTraceEndl(w)
	}
	return writeNumeralDisplayPart(w, part)
}

func (m *ArbitraryFloatManager) traceSetInt32Result(out *ArbitraryFloat) error {
	return m.traceSetOrdinaryResult(out, 105)
}

func (m *ArbitraryFloatManager) traceSetPlainRationalEntry(value *Rational, ebits, sbits uint32) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "set", "src/util/mpf.cpp", 187,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "set: "); e != nil {
				return e
			}
			s, e := m.rational.String(value)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, s); e != nil {
				return e
			}
			for _, part := range [...]string{" [", strconv.FormatUint(uint64(ebits), 10), "/", strconv.FormatUint(uint64(sbits), 10), "]", "\n"} {
				if e := writeSetTracePart(w, part); e != nil {
					return e
				}
			}
			return nil
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceSetExponentRationalEntry(significand *Rational, exponent *Int) error {
	return m.traceSetRationalAndExponent("set: sig = ", significand, exponent, 235)
}

func (m *ArbitraryFloatManager) traceSetExponentRationalNormalized(significand *Rational, exponent *Int) error {
	return m.traceSetRationalAndExponent("Normalized: sig = ", significand, exponent, 270)
}

func (m *ArbitraryFloatManager) traceSetRationalAndExponent(label string, significand *Rational, exponent *Int, line int) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "set", "src/util/mpf.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, label); e != nil {
				return e
			}
			s, e := m.rational.String(significand)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, s); e != nil {
				return e
			}
			if e := writeNumeralDisplayPart(w, " exp = "); e != nil {
				return e
			}
			s, e = m.integer.String(exponent)
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

func (m *ArbitraryFloatManager) traceSetExponentRationalFields(out *ArbitraryFloat) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "set", "src/util/mpf.cpp", 289,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "sig = "); e != nil {
				return e
			}
			s, e := m.integer.String(&out.significand)
			if e != nil {
				formatError = e
				return e
			}
			for _, part := range [...]string{s, " exp = ", strconv.FormatInt(out.exponent, 10), "\n"} {
				if e := writeSetTracePart(w, part); e != nil {
					return e
				}
			}
			return nil
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceSetLiteralEntry(value string, ebits, sbits uint32) {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return
	}
	if i := strings.IndexByte(value, 0); i >= 0 {
		value = value[:i]
	}
	parts := [...]string{"set: ", value, " [", strconv.FormatUint(uint64(ebits), 10), "/", strconv.FormatUint(uint64(sbits), 10), "]", "\n"}
	_ = tags.EmitConditional("mpf_dbg", "set", "src/util/mpf.cpp", 195,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range parts {
				if e := writeSetTracePart(w, part); e != nil {
					return e
				}
			}
			return nil
		})
}

func (m *ArbitraryFloatManager) traceSetLiteralParts(negative bool, fraction, exponent string) {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return
	}
	parts := [...]string{"sgn = ", traceBoolDigit(negative), " f = ", fraction, " e = ", exponent, "\n"}
	_ = tags.EmitConditional("mpf_dbg", "set", "src/util/mpf.cpp", 219,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range parts {
				if e := writeSetTracePart(w, part); e != nil {
					return e
				}
			}
			return nil
		})
}

func (m *ArbitraryFloatManager) traceSetOrdinaryResult(out *ArbitraryFloat, line int) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "set", "src/util/mpf.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "set: res = "); e != nil {
				return e
			}
			value, e := m.formatFloatTraceValue(out)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, value); e != nil {
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}

func traceNativeFloatText(value float64, bits int) string {
	if math.IsNaN(value) {
		return "nan"
	}
	if math.IsInf(value, 1) {
		return "inf"
	}
	if math.IsInf(value, -1) {
		return "-inf"
	}
	return strconv.FormatFloat(value, 'g', 6, bits)
}

func (m *ArbitraryFloatManager) traceSetNativeInput(value float64, bits int, raw, sig uint64, sign bool, exponent int64, line int) {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return
	}
	kind := "double"
	if bits == 32 {
		kind = "float"
	}
	parts := [...]string{"set: ", traceNativeFloatText(value, bits), " is: raw=", strconv.FormatUint(raw, 10), " (", kind, ") sign=", traceBoolDigit(sign), " s=", strconv.FormatUint(sig, 10), " e=", strconv.FormatInt(exponent, 10), "\n"}
	_ = tags.EmitConditional("mpf_dbg", "set", "src/util/mpf.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range parts {
				if e := writeSetTracePart(w, part); e != nil {
					return e
				}
			}
			return nil
		})
}

func (m *ArbitraryFloatManager) traceSetFloatRawResult(out *ArbitraryFloat) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "set", "src/util/mpf.cpp", 183,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "set: res = "); e != nil {
				return e
			}
			formatted, e := m.RawString(out)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, formatted); e != nil {
				return e
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}
