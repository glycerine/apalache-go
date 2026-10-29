package numeral

// Source: src/util/mpq.cpp:185-270.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *RationalManager) parseTraceTags() *diagnostic.TraceTags {
	if m.Z.trace != nil {
		return m.Z.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

// set(char const*) traces before it has established a valid denominator.
// Source to_string prints the raw numerator and denominator in that state.
func (m *RationalManager) parseRawText(a *Rational) (string, error) {
	n, err := m.Z.String(&a.Num)
	if err != nil {
		return "", err
	}
	if isIntegerRational(a) {
		return n, nil
	}
	d, err := m.Z.String(&a.Den)
	if err != nil {
		return "", err
	}
	return n + "/" + d, nil
}

func (m *RationalManager) traceParseBefore(a *Rational) error {
	var formatErr error
	_ = m.parseTraceTags().EmitConditional("mpq_set", "set", "src/util/mpq.cpp", 203,
		func() bool { return true }, func(w io.Writer) error {
			_ = writeNumeralDisplayPart(w, "[before] a: ")
			v, e := m.parseRawText(a)
			if e != nil {
				formatErr = e
				return e
			}
			for _, part := range [...]string{v, "\n"} {
				_ = writeNumeralDisplayPart(w, part)
			}
			return nil
		})
	return formatErr
}

func (m *RationalManager) traceParseExponent(exp uint64, character byte) {
	_ = m.parseTraceTags().EmitConditional("mpq_set", "set", "src/util/mpq.cpp", 241,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{"[exp]: ", strconv.FormatUint(exp, 10), ", str[0]: ", strconv.FormatInt(int64(character)-'0', 10)} {
				_ = writeNumeralDisplayPart(w, part)
			}
			_ = diagnostic.WriteTraceEndl(w)
			return nil
		})
}

func (m *RationalManager) traceParsePower(a *Rational, negative bool, exp uint64, power *Rational) error {
	var formatErr error
	_ = m.parseTraceTags().EmitConditional("mpq_set", "set", "src/util/mpq.cpp", 260,
		func() bool { return true }, func(w io.Writer) error {
			_ = writeNumeralDisplayPart(w, "a: ")
			value, e := m.parseRawText(a)
			if e != nil {
				formatErr = e
				return e
			}
			sign := "0"
			if negative {
				sign = "1"
			}
			for _, part := range [...]string{value, ", exp_sign:", sign, ", exp: ", strconv.FormatUint(exp, 10), " "} {
				_ = writeNumeralDisplayPart(w, part)
			}
			factor, e := m.parseRawText(power)
			if e != nil {
				formatErr = e
				return e
			}
			_ = writeNumeralDisplayPart(w, factor)
			_ = diagnostic.WriteTraceEndl(w)
			return nil
		})
	return formatErr
}
