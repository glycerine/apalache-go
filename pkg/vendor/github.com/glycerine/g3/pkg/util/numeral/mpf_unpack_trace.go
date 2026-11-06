package numeral

// Source: src/util/mpf.cpp:1922-1929.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ArbitraryFloatManager) traceUnpackEntry(a *ArbitraryFloat, normalize bool) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "unpack", "src/util/mpf.cpp", 1923,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "unpack "); e != nil {
				return e
			}
			value, e := m.formatFloatTraceValue(a)
			if e != nil {
				formatError = e
				return e
			}
			for _, part := range [...]string{value, ": ebits=", strconv.FormatUint(uint64(a.EBits()), 10),
				" sbits=", strconv.FormatUint(uint64(a.SBits()), 10), " normalize=", traceBoolDigit(normalize)} {
				if e := writeNumeralDisplayPart(w, part); e != nil {
					return e
				}
			}
			top, e := m.hasTopExponent(a)
			if e != nil {
				formatError = e
				return e
			}
			for _, part := range [...]string{" has_top_exp=", traceBoolDigit(top), " ("} {
				if e := writeNumeralDisplayPart(w, part); e != nil {
					return e
				}
			}
			topExp, e := m.topExponent(a.EBits())
			if e != nil {
				formatError = e
				return e
			}
			for _, part := range [...]string{strconv.FormatInt(topExp, 10), ")"} {
				if e := writeNumeralDisplayPart(w, part); e != nil {
					return e
				}
			}
			bottom, e := m.hasBottomExponent(a)
			if e != nil {
				formatError = e
				return e
			}
			for _, part := range [...]string{" has_bot_exp=", traceBoolDigit(bottom), " ("} {
				if e := writeNumeralDisplayPart(w, part); e != nil {
					return e
				}
			}
			bottomExp, e := m.bottomExponent(a.EBits())
			if e != nil {
				formatError = e
				return e
			}
			for _, part := range [...]string{strconv.FormatInt(bottomExp, 10), ")"} {
				if e := writeNumeralDisplayPart(w, part); e != nil {
					return e
				}
			}
			zero, e := m.IsZero(a)
			if e != nil {
				formatError = e
				return e
			}
			for _, part := range [...]string{" is_zero=", traceBoolDigit(zero)} {
				if e := writeNumeralDisplayPart(w, part); e != nil {
					return e
				}
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}
