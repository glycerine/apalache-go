package numeral

// Source: src/util/mpf.cpp:1214-1217,1225,1252.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ArbitraryFloatManager) traceSBVDecision(x *ArbitraryFloat, z *Int, last, round, sticky, increment bool) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_sbv") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg_sbv", "to_sbv_mpq", "src/util/mpf.cpp", 1214,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "SBV: ("); e != nil {
				return e
			}
			value, e := m.formatFloatTraceValue(x)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, value+") == "); e != nil {
				return e
			}
			integer, e := m.integer.String(z)
			if e != nil {
				formatError = e
				return e
			}
			if e := writeNumeralDisplayPart(w, integer); e != nil {
				return e
			}
			if e := diagnostic.WriteTraceEndl(w); e != nil {
				return e
			}
			for _, part := range [...]string{"sign=", traceBoolDigit(x.Sign()), " last=", traceBoolDigit(last), " round=", traceBoolDigit(round), " sticky=", traceBoolDigit(sticky), " inc=", traceBoolDigit(increment)} {
				if e := writeNumeralDisplayPart(w, part); e != nil {
					return e
				}
			}
			return diagnostic.WriteTraceEndl(w)
		})
	return formatError
}

func (m *ArbitraryFloatManager) traceSBVResult(out *Rational) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg", "to_sbv_mpq", "src/util/mpf.cpp", 1225,
		func() bool { return true }, func(w io.Writer) error {
			if e := writeNumeralDisplayPart(w, "SBV = "); e != nil {
				return e
			}
			s, e := m.rational.String(out)
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
