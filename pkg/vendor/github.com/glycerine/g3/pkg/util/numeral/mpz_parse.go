package numeral

// Source: src/util/mpz.cpp:283-303; src/util/mpz.h:692.

import "github.com/glycerine/g3/pkg/util/diagnostic"

// numeralByte gives the finite Go-string boundary the source C-string's NUL
// semantics. Embedded NUL ends parsing; bytes past it are never consumed.
func numeralByte(text string, i int) byte {
	if i >= len(text) {
		return 0
	}
	return text[i]
}

// SetString preserves the source caller RESET, direct digit publication and
// RAW temporary's end-only DEL. Input is borrowed only for this call.
// Failed bodies retain actual RAW work; CleanupStringSet joins that work.
func (m *IntegerManager) SetString(out *Int, text string) error {
	return m.setStringWithBeginCapture(out, text, nil, nil)
}
func (m *IntegerManager) setStringWithBeginCapture(out *Int, text string, frame **integerStringSetScope, invocation *uint64) (cause error) {
	if e := m.check(out); e != nil {
		return e
	}
	s := &m.stringSet
	if s.active || s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = integerStringSetScope{manager: m, invocation: s.invocation + 1, active: true, executing: true}
	if frame != nil {
		*frame = s
	}
	if invocation != nil {
		*invocation = s.invocation
	}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, s.sourceCause)
		cause = JoinNumericErrors(cause, s.unbind())
		if cause == nil {
			s.active = false
		}
	}()
	// RESET precedes the two RAW declarations; a release failure constructs neither.
	if _, e := SourceNumeralDel(m, out); e != nil {
		s.sourceCause = e
		return e
	}
	out.Val = 0
	s.ten = NewInt(10)
	s.declared = true
	c := integerAllocationInvocationContext{Caller: &s.hooks}
	for numeralByte(text, s.index) == ' ' {
		s.index++
	}
	s.negative = numeralByte(text, s.index) == '-'
	for numeralByte(text, s.index) != 0 {
		ch := numeralByte(text, s.index)
		if ch >= '0' && ch <= '9' {
			if modularSourceDebug {
				d := diagnostic.BorrowProcessDebugContext()
				enabled, e := diagnostic.SourceAssertionsEnabled(d)
				if e != nil {
					return e
				}
				if enabled && ch-'0' > 9 {
					if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 294, "str[0] - '0' <= 9"); e != nil {
						return e
					}
				}
			}
			s.stage = 1
			if e := m.mulWithAllocationContext(out, &s.ten, &s.tmp, c); e != nil {
				return e
			}
			digit := NewInt(int32(ch - '0'))
			s.stage = 2
			if e := m.addSubWithAllocationContext(&s.tmp, &digit, out, false, c); e != nil {
				return e
			}
		}
		s.index++
	}
	s.stage = 3
	s.endReached = true
	if _, e := SourceNumeralDel(m, &s.tmp); e != nil {
		s.sourceCause = e
		return e
	}
	s.tmpDelDone = true
	s.stage = 4
	if s.negative {
		if e := m.negWithAllocationContext(out, c); e != nil {
			return e
		}
	}
	if e := s.cleanupArithmetic(); e != nil {
		return e
	}
	s.bodySucceeded = true
	return nil
}
