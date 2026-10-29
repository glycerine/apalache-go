package numeral

// Source: src/util/mpq.cpp set(mpq &, char const *), all source branches.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrMixedNotation = errors.New("mpnum: mixing rational/scientific notation")
var ErrExponentTooLarge = errors.New("mpnum: exponent is too big")

// The two reached digit guards in mpq_manager::set have the same source
// assertion but distinct locations and occur before distinct mutations.
func rationalParserDigitAssertion(b byte, line int32) error {
	if !modularSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil {
		return e
	}
	if enabled && b-'0' > 9 {
		return diagnostic.DebugAssertionFailure(d, "src/util/mpq.cpp", line, "str[0] - '0' <= 9")
	}
	return nil
}

// The three parser-owned rational values have fixed source declaration order:
// b, _exp, _ten. Their destructors run in the reverse order.
type rationalParserScopedValue struct {
	value   Rational
	stage   uint8 // 0: numerator, 1: denominator
	pending bool
}

type ParseErrorReason uint8

const (
	ParseMixedNotation ParseErrorReason = iota + 1
	ParseExponentTooLarge
	ParseZeroDenominator
)

// ParseError classifies the three actual source parser exceptions. Allocation,
// owner and nonterminating-Power failures retain their distinct error types.
type ParseError struct{ Reason ParseErrorReason }

func (e *ParseError) Unwrap() error {
	switch e.Reason {
	case ParseMixedNotation:
		return ErrMixedNotation
	case ParseExponentTooLarge:
		return ErrExponentTooLarge
	default:
		return ErrDivideByZero
	}
}
func (e *ParseError) Message() string {
	switch e.Reason {
	case ParseMixedNotation:
		return "mixing rational/scientific notation"
	case ParseExponentTooLarge:
		return "exponent is too big"
	default:
		return "division by zero"
	}
}
func (e *ParseError) Error() string                    { return e.Message() }
func (e *ParseError) ErrorCode() z3exception.ErrorCode { return z3exception.ErrorOK }
func (e *ParseError) HasErrorCode() bool               { return false }
func (e *ParseError) As(target any) bool {
	base, ok := target.(*z3exception.DefaultException)
	if !ok {
		return false
	}
	*base = z3exception.NewDefaultException(e.Message())
	return true
}

// SetString explicitly initializes/reinitializes owned integer fields, including
// a zero denominator. Ordinary arithmetic still rejects that raw state. The
// source parser publishes numerator reset and each digit/component action into
// the caller; errors retain those completed actions. Scientific input without
// a dot keeps the actual old denominator until its later source arithmetic.
func (m *RationalManager) SetString(out *Rational, text string) (err error) {
	if m == nil {
		return umemory.ErrRuntimeOwner
	}
	if m.parseOuterPending || m.parseSuffixPending || m.parseB.pending || m.parseExp.pending || m.parseTen.pending {
		return ErrRationalLexicalPending
	}
	if m.closed {
		return arena.ErrClosed
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	// Ordinary checks reject stale/failed references before Reset. Cleanup-only
	// acceptance must not authorize arithmetic on a failed owning descriptor.
	if e := m.Z.check(&out.Num); e != nil {
		return e
	}
	if e := m.Z.check(&out.Den); e != nil {
		return e
	}
	if e := m.Z.Reset(&out.Num); e != nil {
		return e
	}
	ten := NewInt(10)
	var tmp Int
	defer func() {
		if _, cleanupErr := sourceNumeralDel(m.Z, &tmp); cleanupErr != nil {
			m.parseOuterTemporary = tmp
			m.parseOuterPending = true
			if err == nil {
				err = cleanupErr
			}
		}
	}()
	i := 0
	for numeralByte(text, i) == ' ' {
		i++
	}
	negative := numeralByte(text, i) == '-'
	for {
		b := numeralByte(text, i)
		if b == 0 || b == '/' || b == '.' || b == 'e' || b == 'E' {
			break
		}
		if b >= '0' && b <= '9' {
			if e := rationalParserDigitAssertion(b, 197); e != nil {
				return e
			}
			digit := NewInt(int32(b - '0'))
			if e := m.Z.Mul(&out.Num, &ten, &tmp); e != nil {
				return e
			}
			if e := m.Z.Add(&tmp, &digit, &out.Num); e != nil {
				return e
			}
		}
		i++
	}
	marker := numeralByte(text, i)
	if e := m.traceParseBefore(out); e != nil {
		return e
	}
	if marker == '/' || marker == '.' || marker == 'e' || marker == 'E' {
		if e := m.parseRationalSuffix(out, text, i, &tmp, &ten); e != nil {
			return e
		}
	} else {
		if e := m.resetDenominator(out); e != nil {
			return e
		}
	}
	if negative {
		if e := m.Z.Neg(&out.Num); e != nil {
			return e
		}
	}
	if e := m.normalize(out); e != nil {
		return e
	}
	return nil
}

// A separate lexical scope keeps source suffix temporaries alive exactly until
// the suffix branch exits, before the outer sign adjustment/normalization.
func (m *RationalManager) parseRationalSuffix(work *Rational, text string, i int, tmp, ten *Int) (err error) {
	marker := numeralByte(text, i)

	isRat := marker == '/'
	tmp2 := NewInt(1)
	defer func() {
		if _, cleanupErr := sourceNumeralDel(m.Z, &tmp2); cleanupErr != nil {
			m.parseSuffixTemporary = tmp2
			m.parseSuffixPending = true
			if err == nil {
				err = cleanupErr
			}
		}
	}()
	hasDen := false
	if marker == '/' || marker == '.' {
		hasDen = true
		i++
		if e := m.Z.Reset(&work.Den); e != nil {
			return e
		}
		for {
			b := numeralByte(text, i)
			if b == 0 || b == 'e' || b == 'E' {
				break
			}
			if b >= '0' && b <= '9' {
				digit := NewInt(int32(b - '0'))
				if e := m.Z.Mul(&work.Den, ten, tmp); e != nil {
					return e
				}
				if e := m.Z.Add(tmp, &digit, &work.Den); e != nil {
					return e
				}
				if !isRat {
					if e := m.Z.Mul(&tmp2, ten, &tmp2); e != nil {
						return e
					}
				}
			}
			i++
		}
	}
	var exponent uint64
	expNegative := false
	if b := numeralByte(text, i); b == 'e' || b == 'E' {
		if isRat {
			return &ParseError{Reason: ParseMixedNotation}
		}
		i++
		if numeralByte(text, i) == '-' {
			expNegative = true
			i++
		}
		for ; numeralByte(text, i) != 0; i++ {
			b := numeralByte(text, i)
			if b >= '0' && b <= '9' {
				if e := rationalParserDigitAssertion(b, 235); e != nil {
					return e
				}
				exponent = 10*exponent + uint64(b-'0')
			} else if b == '/' {
				return &ParseError{Reason: ParseMixedNotation}
			}
			m.traceParseExponent(exponent, b)
		}
	}
	if !isRat {
		if exponent > uint64(^uint32(0)) {
			return &ParseError{Reason: ParseExponentTooLarge}
		}
		b := NewRational()
		defer func() {
			if cleanupErr := m.finishParserRationalSource(&b, &m.parseB); cleanupErr != nil && err == nil {
				err = cleanupErr
			}
		}()
		if hasDen {
			if e := m.SetParts(&b, &work.Den, &tmp2); e != nil {
				return e
			}
			if e := m.Z.SetInt64(&work.Den, 1); e != nil {
				return e
			}
			if e := m.Add(work, &b, work); e != nil {
				return e
			}
		}
		if exponent > 0 {
			exp, tenRat := NewRational(), NewRational()
			defer func() {
				if cleanupErr := m.finishParserRationalSource(&exp, &m.parseExp); cleanupErr != nil && err == nil {
					err = cleanupErr
				}
			}()
			defer func() {
				if cleanupErr := m.finishParserRationalSource(&tenRat, &m.parseTen); cleanupErr != nil && err == nil {
					err = cleanupErr
				}
			}()
			if e := m.SetInt64(&tenRat, 10); e != nil {
				return e
			}
			if e := m.Power(&tenRat, uint32(exponent), &exp); e != nil {
				return e
			}
			if e := m.traceParsePower(work, expNegative, exponent, &exp); e != nil {
				return e
			}
			if expNegative {
				if e := m.Div(work, &exp, work); e != nil {
					return e
				}
			} else {
				if e := m.Mul(work, &exp, work); e != nil {
					return e
				}
			}
		}
	} else if work.Den.Val == 0 {
		return &ParseError{Reason: ParseZeroDenominator}
	}
	return nil
}

func (m *RationalManager) finishParserRationalSource(value *Rational, slot *rationalParserScopedValue) error {
	if _, e := sourceNumeralDel(m.Z, &value.Num); e != nil {
		*slot = rationalParserScopedValue{value: *value, pending: true}
		return e
	}
	if _, e := sourceNumeralDel(m.Z, &value.Den); e != nil {
		*slot = rationalParserScopedValue{value: *value, stage: 1, pending: true}
		return e
	}
	return nil
}

func (m *RationalManager) cleanupParserRational(slot *rationalParserScopedValue) error {
	if !slot.pending {
		return nil
	}
	if slot.stage == 0 {
		if e := m.Z.Del(&slot.value.Num); e != nil {
			return e
		}
		slot.stage = 1
	}
	if e := m.Z.Del(&slot.value.Den); e != nil {
		return e
	}
	*slot = rationalParserScopedValue{}
	return nil
}

// CleanupParserTemporaries continues stopped source destructors in reverse
// declaration order, followed by suffix then outer integer scratch storage.
func (m *RationalManager) CleanupParserTemporaries() error {
	if m == nil || m.self != m || m.Z == nil {
		return umemory.ErrRuntimeOwner
	}
	for _, slot := range [...]*rationalParserScopedValue{&m.parseTen, &m.parseExp, &m.parseB} {
		if e := m.cleanupParserRational(slot); e != nil {
			return e
		}
	}
	if m.parseSuffixPending {
		if e := m.Z.Del(&m.parseSuffixTemporary); e != nil {
			return e
		}
		m.parseSuffixTemporary = Int{}
		m.parseSuffixPending = false
	}
	if m.parseOuterPending {
		if e := m.Z.Del(&m.parseOuterTemporary); e != nil {
			return e
		}
		m.parseOuterTemporary = Int{}
		m.parseOuterPending = false
	}
	return nil
}
