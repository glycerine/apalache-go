package numeral

// Source: src/util/mpf.cpp:1795-1819; src/util/mpf.h:183.

import

// The reached scoped_mpz is actual storage in its parent owner. Its captured
// SET receipt identifies the precise younger allocation; diagnostic metadata
// and cached powers are not used as substitute temporary owners.
umemory "github.com/glycerine/g3/pkg/util/memory"

type arbitraryFloatIntegerQueryScope struct {
	t                                         Int
	copy                                      integerCopyAllocationReceipt
	constructed, active, executing, finishing bool
	bodyCause, sourceCause                    error
}

func (m *ArbitraryFloatManager) IsInt(x *ArbitraryFloat) (result bool, cause error) {
	if e := m.valueGuard(x); e != nil {
		return false, e
	}
	if m.integerQuery.active {
		return false, ErrNumericPending
	}
	normal, e := m.IsNormal(x)
	if e != nil || !normal {
		return false, e
	}
	if x.exponent >= int64(x.SBits()-1) {
		return true, nil
	}
	if x.exponent < 0 {
		return false, nil
	}
	if e := mpfSourceAssertion(x.exponent >= 0 && x.exponent < int64(x.SBits()-1), 1805, "x.exponent >= 0 && x.exponent < x.sbits-1"); e != nil {
		return false, e
	}
	s := &m.integerQuery
	*s = arbitraryFloatIntegerQueryScope{constructed: true, active: true, executing: true}
	m.valueExecuting = true
	defer m.completeIntegerQuery(&result, &cause)
	z := m.integer
	if e := z.CopyFromOwnerWithReceipt(&s.t, z, &x.significand, &s.copy); e != nil {
		return false, e
	}
	shift := x.SBits() - uint32(x.exponent) - 1
	for {
		odd, e := z.IsOdd(&s.t)
		if e != nil {
			return false, e
		}
		if odd {
			return false, nil
		}
		if e := z.MachineDiv2kInPlace(&s.t, 1); e != nil {
			return false, e
		}
		shift--
		if shift == 0 {
			return true, nil
		}
	}
}

func (m *ArbitraryFloatManager) completeIntegerQuery(result *bool, cause *error) {
	s := &m.integerQuery
	m.valueExecuting = false
	s.executing = false
	s.bodyCause = *cause
	*cause = JoinNumericErrors(*cause, m.finishIntegerQuerySource())
	if *cause != nil {
		*result = false
	}
}

func (m *ArbitraryFloatManager) finishIntegerQuerySource() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.integerQuery
	if s.executing || s.finishing {
		return ErrNumericPending
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if s.constructed {
		if _, e := SourceNumeralDel(m.integer, &s.t); e != nil {
			s.sourceCause = e
			return e
		}
		s.constructed = false
	}
	s.active = s.bodyCause != nil || m.integer.MatchesCopyAllocationInvocation(&s.copy)
	return nil
}

// CleanupIntegerQuery joins only the captured SET allocation and actual local
// numeral. It never replays the query or changes the const input.
func (m *ArbitraryFloatManager) CleanupIntegerQuery() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.integerQuery
	if s.executing || s.finishing || m.valueExecuting || m.memory != nil && m.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	if !s.active {
		return nil
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if e := m.integer.CleanupCopyAllocationInvocation(&s.copy); e != nil {
		return e
	}
	if s.constructed {
		if e := m.integer.CleanupRawNumeral(&s.t); e != nil {
			return e
		}
		s.constructed = false
	}
	s.active = false
	return nil
}
