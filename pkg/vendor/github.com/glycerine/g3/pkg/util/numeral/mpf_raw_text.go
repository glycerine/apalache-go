package numeral

// Source: src/util/mpf.cpp:1590-1606.

import (
	"strconv"
	"strings"
)

// RawString formats the retained MPF fields with ordinary Go strings. The
// significand still goes through the concrete integer manager's source
// conversion path, including its numerical failure and lifetime guards.
func (m *ArbitraryFloatManager) RawString(x *ArbitraryFloat) (string, error) {
	if e := m.valueGuard(x); e != nil {
		return "", e
	}
	formatInvocation := m.integer.Natural.Formatting.invocation
	characterInvocation := m.integer.Natural.CharFormatting.invocation
	significand, e := m.integer.String(&x.significand)
	if e != nil {
		natural := m.integer.Natural
		if natural.Formatting.invocation != formatInvocation || natural.CharFormatting.invocation != characterInvocation {
			e = JoinNumericErrors(e, natural.cleanupOutputFormatting())
		}
		return "", e
	}
	normal, e := m.IsNormal(x)
	if e != nil {
		return "", e
	}
	var b strings.Builder
	b.Grow(len(significand) + 27)
	b.WriteByte('[')
	if x.Sign() {
		b.WriteByte('-')
	} else {
		b.WriteByte('+')
	}
	b.WriteByte(' ')
	b.WriteString(significand)
	b.WriteByte(' ')
	b.WriteString(strconv.FormatInt(x.exponent, 10))
	if normal {
		b.WriteString(" N]")
	} else {
		b.WriteString(" D]")
	}
	return b.String(), nil
}
