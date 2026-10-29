package numeral

// Source: src/util/mpz.h:468-485; src/util/mpz.cpp:915-988.

type integerScalarRawScope struct {
	self                                         *integerScalarRawScope
	manager                                      *IntegerManager
	parent                                       *NumericInvocationHooks
	invocation                                   uint64
	ActiveField, executing, BodySucceeded, ended bool
	Values                                       [6]Int
	Count, cursor                                uint8
	sourceCause                                  error
}

func (m *IntegerManager) scalarRawFrameAddress(s *integerScalarRawScope, r *NumericInvocationHooks) bool {
	return m != nil && m.self == m && s != nil && (s.self == nil || s.self == s) && r != nil && (r.Policy != nil || r.Roots != nil) && r.callBound(false)
}
func (m *IntegerManager) scalarRawParentIdentity(s *integerScalarRawScope, r *NumericInvocationHooks) bool {
	return m.scalarRawFrameAddress(s, r) && s.manager == m && s.parent == r
}
func (m *IntegerManager) BeginScalarRaw(s *integerScalarRawScope, r *NumericInvocationHooks, count uint8) error {
	if !m.scalarRawFrameAddress(s, r) || !r.callBound(true) || r.callRoots(m, nil) == nil {
		return ErrNumericOwner
	}
	if s.ActiveField || s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = integerScalarRawScope{self: s, manager: m, parent: r, invocation: s.invocation + 1, ActiveField: true, executing: true, Count: count}
	return nil
}
func (m *IntegerManager) FinishScalarRawSource(s *integerScalarRawScope) error {
	if s == nil || !m.scalarRawParentIdentity(s, s.parent) {
		return ErrNumericOwner
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.ActiveField || !s.BodySucceeded || s.ended {
		return nil
	}
	for s.cursor < s.Count {
		_, e := SourceNumeralDel(m, &s.Values[s.cursor])
		if e != nil {
			s.sourceCause = e
			return e
		}
		s.cursor++
	}
	s.ended = true
	s.ActiveField = false
	return nil
}
func (m *IntegerManager) CleanupScalarRaw(s *integerScalarRawScope, r *NumericInvocationHooks, abort bool) error {
	if !m.scalarRawFrameAddress(s, r) {
		return ErrNumericOwner
	}
	if !m.scalarRawParentIdentity(s, r) || !r.callBound(false) || !s.ActiveField {
		return nil
	}
	if s.executing {
		return ErrNumericPending
	}
	if !abort && !s.BodySucceeded {
		return nil
	}
	for s.cursor < s.Count {
		if e := m.Del(&s.Values[s.cursor]); e != nil {
			return e
		}
		s.cursor++
	}
	s.ActiveField = false
	return nil
}

func (s *integerScalarRawScope) Cleanup(abort bool) error {
	if s == nil {
		return nil
	}
	if s.manager == nil && s.self == nil && s.parent == nil && s.invocation == 0 && !s.ActiveField {
		return nil
	}
	if s.manager == nil {
		return ErrNumericOwner
	}
	return s.manager.CleanupScalarRaw(s, s.parent, abort)
}

func (s *integerScalarRawScope) SourceCause() error { return s.sourceCause }
func (s *integerScalarRawScope) Active() bool       { return s.ActiveField }
