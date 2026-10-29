package numeral

// Source: src/util/mpz.cpp:2054-2069; src/util/scoped_numeral.h:31-33.

// The actual Integer method owns its scoped tmp NUM, including the zero
// branch. Shared division buffers remain manager-owned; checked arithmetic
// verification uses native scratch. There is no RAW end-only DEL in this method.
type integerDividesNumeralScope struct {
	owner                                        *IntegerManager
	Receipt                                      integerDividesNumeralInvocationReceipt
	ParentContext                                integerAllocationInvocationContext
	Invocation                                   uint64
	Active, Executing, Constructed, Ended, owned bool
	Numeral                                      Int
	Numeric                                      NumericAllocationRoots

	BodyCause, sourceCause error
}
type integerDividesNumeralInvocationReceipt struct {
	Manager    *IntegerManager
	Frame      *integerDividesNumeralScope
	Invocation uint64
}

func (m *IntegerManager) BeginDividesNumeralInvocation(parent integerAllocationInvocationContext) (*integerDividesNumeralInvocationReceipt, error) {
	if m == nil || m.self != m || m.closed || m.SourceFinalizing || m.constructionFailed {
		return nil, ErrNumericOwner
	}
	if parent != (integerAllocationInvocationContext{}) && parent.roots(m) == nil {
		return nil, ErrNumericOwner
	}
	s := &m.DividesNumeral
	if s.Active || s.Invocation == ^uint64(0) {
		return nil, ErrNumericPending
	}
	*s = integerDividesNumeralScope{owner: m, Invocation: s.Invocation + 1, Active: true, Executing: true, Constructed: true, ParentContext: parent}
	s.Receipt = integerDividesNumeralInvocationReceipt{Manager: m, Frame: s, Invocation: s.Invocation}
	parent.recordCaller(m, "divides", NumericInvocationReceipt{Divides: &s.Receipt})
	return &s.Receipt, nil
}

func (m *IntegerManager) CleanupDividesNumeral() error {
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	if e := m.CleanupDividesNumeralInvocation(&m.DividesNumeral.Receipt, false); e != nil {
		return e
	}
	return m.CleanupDividesNumeralInvocation(&m.DividesNumeral.Receipt, true)
}

func (m *IntegerManager) matchesDividesNumeralHistory(r *integerDividesNumeralInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.Manager == m && r.Frame == &m.DividesNumeral && r.Frame.owner == m && r.Invocation != 0 && r.Invocation == r.Frame.Invocation
}
func (m *IntegerManager) MatchesDividesNumeralInvocation(r *integerDividesNumeralInvocationReceipt) bool {
	return m.matchesDividesNumeralHistory(r) && r.Frame.Active
}
func (m *IntegerManager) dividesNumeralContextRoots(c integerAllocationInvocationContext) *NumericAllocationRoots {
	r := c.BoolDivides
	if c.BoolDividesInvocation == 0 || r == nil || c.BoolDividesInvocation != r.Invocation || !m.MatchesDividesNumeralInvocation(r) || !r.Frame.Executing || m.closed || m.SourceFinalizing || m.constructionFailed {
		return nil
	}
	if p := r.Frame.ParentContext; p != (integerAllocationInvocationContext{}) && p.roots(m) == nil {
		return nil
	}
	return &r.Frame.Numeric
}
func (m *IntegerManager) DividesNumeralInvocationSourceCause(r *integerDividesNumeralInvocationReceipt) error {
	if m.matchesDividesNumeralHistory(r) {
		return r.Frame.sourceCause
	}
	return nil
}
func (m *IntegerManager) FinishDividesNumeralSource(s *integerDividesNumeralScope) error {
	if m == nil || m.self != m || s != &m.DividesNumeral || s.owner != m {
		return ErrNumericOwner
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}

	if !s.Active || !s.Constructed || s.Ended || s.owned {
		return nil
	}
	if _, e := SourceNumeralDel(m, &s.Numeral); e != nil {
		s.sourceCause = e
		return e
	}
	s.Ended = true
	if !s.Pending(m) {
		s.Active = false
	}
	return nil
}
func (s *integerDividesNumeralScope) Pending(m *IntegerManager) bool {
	if NumericAllocationPending(&s.Numeric) {
		return true
	}

	return false
}
func (m *IntegerManager) CleanupDividesNumeralInvocation(r *integerDividesNumeralInvocationReceipt, abort bool) error {
	if !m.MatchesDividesNumeralInvocation(r) {
		return nil
	}
	s := r.Frame
	if s.Executing {
		return ErrNumericPending
	}

	if s.Constructed && !s.Ended && !s.owned {
		if e := m.CleanupRawNumeral(&s.Numeral); e != nil {
			return e
		}
		s.owned = true
	}
	if abort {

		if e := s.Numeric.Cleanup(m); e != nil {
			return e
		}
	}
	if !s.Pending(m) {
		s.Active = false
	}
	return nil
}
