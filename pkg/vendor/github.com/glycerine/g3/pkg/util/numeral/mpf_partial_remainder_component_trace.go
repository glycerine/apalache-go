package numeral

// Source: src/util/mpf.cpp:1301-1304,1331-1333,1352-1355,1410-1415.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ArbitraryFloatManager) formatPartialRemainderComponents(sign bool, exponent int64, significand *Int, ebits, sbits, extra uint32) (result string, cause error) {
	text, e := NewArbitraryFloatFormatText(m.budget)
	if e != nil {
		return "", e
	}
	defer func() {
		cause = JoinNumericErrors(cause, text.Close())
		if cause != nil {
			result = ""
		}
	}()
	view, e := m.ToStringHexfloatComponentsInto(sign, exponent, significand, ebits, sbits, extra, text)
	if e != nil {
		return "", JoinNumericErrors(e, m.CleanupHexfloatComponents())
	}
	n, e := view.Length()
	if e != nil {
		return "", e
	}
	if n > uint64(int(^uint(0)>>1)) {
		return "", arena.ErrBounds
	}
	data := make([]byte, int(n))
	for i := range data {
		data[i], e = view.Byte(uint64(i))
		if e != nil {
			return "", e
		}
	}
	return string(data), nil
}

func (m *ArbitraryFloatManager) writePartialRemainderTraceInt(w io.Writer, label string, value *Int, formatError *error) error {
	if e := writeNumeralDisplayPart(w, label); e != nil {
		return e
	}
	s, e := m.integer.String(value)
	if e != nil {
		*formatError = e
		return e
	}
	if e := writeNumeralDisplayPart(w, s); e != nil {
		return e
	}
	return diagnostic.WriteTraceEndl(w)
}

func (m *ArbitraryFloatManager) writePartialRemainderTraceComponents(w io.Writer, label string, sign bool, exponent int64, significand *Int, ebits, sbits, extra uint32, formatError *error) error {
	if e := writeNumeralDisplayPart(w, label); e != nil {
		return e
	}
	s, e := m.formatPartialRemainderComponents(sign, exponent, significand, ebits, sbits, extra)
	if e != nil {
		*formatError = e
		return e
	}
	if e := writeNumeralDisplayPart(w, s); e != nil {
		return e
	}
	return diagnostic.WriteTraceEndl(w)
}

func (m *ArbitraryFloatManager) tracePartialRemainderRatio(sign bool, exponent int64, large, rem *Int, ebits, sbits uint32) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_rem") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg_rem", "partial_remainder", "src/util/mpf.cpp", 1301,
		func() bool { return true }, func(w io.Writer) error {
			if e := writePartialRemainderScalarLine(w, "X/Y_exp=", strconv.FormatInt(exponent, 10)); e != nil {
				return e
			}
			if e := m.writePartialRemainderTraceInt(w, "X/Y_sig_lrg=", large, &formatError); e != nil {
				return e
			}
			if e := m.writePartialRemainderTraceInt(w, "X/Y_rem=", rem, &formatError); e != nil {
				return e
			}
			return m.writePartialRemainderTraceComponents(w, "X/Y~=", sign, exponent, large, ebits, sbits, sbits+3, &formatError)
		})
	return formatError
}

func (m *ArbitraryFloatManager) tracePartialRemainderQ(sign bool, exponent int64, significand *Int, ebits, sbits uint32) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_rem") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg_rem", "partial_remainder", "src/util/mpf.cpp", 1331,
		func() bool { return true }, func(w io.Writer) error {
			if e := writePartialRemainderScalarLine(w, "Q_exp=", strconv.FormatInt(exponent, 10)); e != nil {
				return e
			}
			if e := m.writePartialRemainderTraceInt(w, "Q_sig=", significand, &formatError); e != nil {
				return e
			}
			return m.writePartialRemainderTraceComponents(w, "Q=", sign, exponent, significand, ebits, sbits, 0, &formatError)
		})
	return formatError
}

func (m *ArbitraryFloatManager) tracePartialRemainderProduct(sign bool, exponent int64, significand *Int, ebits, sbits uint32) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_rem") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg_rem", "partial_remainder", "src/util/mpf.cpp", 1352,
		func() bool { return true }, func(w io.Writer) error {
			if e := writePartialRemainderScalarLine(w, "YQ_sgn=", traceBoolDigit(sign)); e != nil {
				return e
			}
			if e := writePartialRemainderScalarLine(w, "YQ_exp=", strconv.FormatInt(exponent, 10)); e != nil {
				return e
			}
			if e := m.writePartialRemainderTraceInt(w, "YQ_sig=", significand, &formatError); e != nil {
				return e
			}
			return m.writePartialRemainderTraceComponents(w, "YQ=", sign, exponent, significand, ebits, sbits, sbits-1, &formatError)
		})
	return formatError
}

func (m *ArbitraryFloatManager) tracePartialRemainderDifference(minuend, subtrahend *Int, sign bool, exponent int64, significand *Int, ebits, sbits uint32) error {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg_rem") {
		return nil
	}
	var formatError error
	_ = tags.EmitConditional("mpf_dbg_rem", "partial_remainder", "src/util/mpf.cpp", 1410,
		func() bool { return true }, func(w io.Writer) error {
			if e := m.writePartialRemainderTraceInt(w, "minuend=", minuend, &formatError); e != nil {
				return e
			}
			if e := m.writePartialRemainderTraceInt(w, "subtrahend=", subtrahend, &formatError); e != nil {
				return e
			}
			if e := writePartialRemainderScalarLine(w, "X_YQ_sgn=", traceBoolDigit(sign)); e != nil {
				return e
			}
			if e := writePartialRemainderScalarLine(w, "X_YQ_exp=", strconv.FormatInt(exponent, 10)); e != nil {
				return e
			}
			if e := m.writePartialRemainderTraceInt(w, "X_YQ_sig=", significand, &formatError); e != nil {
				return e
			}
			return m.writePartialRemainderTraceComponents(w, "X-YQ=", sign, exponent, significand, ebits, sbits, sbits-1, &formatError)
		})
	return formatError
}
