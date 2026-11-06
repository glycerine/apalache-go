package numeral

// Source: src/util/mpz.h:468-485; src/util/mpz.cpp:730-817,915-985,1018-1183.

// Real retained RAW declarations in the charged Integer owner. Count marks
// reached declarations; values beyond count carry no source constructor credit.
type IntegerRawScope struct {
	caller *NumericInvocationHooks

	rationalSetContext integerAllocationInvocationContext
	formatContext      integerAllocationInvocationContext
	conversionContext  integerAllocationInvocationContext

	AddMul integerAddMulInvocationReceipt

	owner                                          *IntegerManager
	receipt                                        integerRawInvocationReceipt
	Invocation                                     uint64
	Active, executing, BodySucceeded, ended, owned bool
	Values                                         [6]Int
	Count, cursor                                  uint8
	Numeric                                        NumericAllocationRoots

	BodyCause, sourceCause error
}
type integerRawInvocationReceipt struct {
	Manager    *IntegerManager
	Frame      *IntegerRawScope
	Invocation uint64
}

// Executing RAW and scoped Divides NUM calls retain their real declarations.
// Public owner destruction cannot run from a returning error transport callback.
func (m *IntegerManager) rawInvocationExecuting() bool {
	return m.numericCallerExecuting() || m.rationalServiceReadCallbackActive() || m.conversionCallerExecuting() || (m.MpfParent != nil && m.MpfParent.formatInvocationExecuting()) || m.mpfBorrowedSourceCallbackActive() || m.mpfLifecycleCallbackActive() || (m.DivisionRaw.Active && m.DivisionRaw.executing) ||
		(m.ExtendedRaw.Active && m.ExtendedRaw.executing) ||
		(m.GcdRaw.Active && m.GcdRaw.executing) ||
		(m.BitwiseNotRaw.Active && m.BitwiseNotRaw.executing) ||
		(m.BitwiseOrRaw.Active && m.BitwiseOrRaw.executing) ||
		(m.BitwiseAndRaw.Active && m.BitwiseAndRaw.executing) ||
		(m.BitwiseXorRaw.Active && m.BitwiseXorRaw.executing) ||
		(m.AddMulInvocation.Active && m.AddMulInvocation.executing) ||
		(m.DividesNumeral.Active && m.DividesNumeral.Executing)
}

func (m *IntegerManager) rawFrameAddress(s *IntegerRawScope) bool {
	return m != nil && m.self == m && s != nil && (s == &m.DivisionRaw || s == &m.ExtendedRaw || s == &m.GcdRaw || s == &m.BitwiseNotRaw || s == &m.BitwiseOrRaw || s == &m.BitwiseAndRaw || s == &m.BitwiseXorRaw)
}
func (m *IntegerManager) MatchesRawInvocation(r *integerRawInvocationReceipt) bool {
	return r != nil && r.Manager == m && m.rawFrameAddress(r.Frame) && r.Frame.owner == m && r.Invocation != 0 && r.Invocation == r.Frame.Invocation && r.Frame.Active
}
func (m *IntegerManager) RawAllocationRoots(r *integerRawInvocationReceipt) *NumericAllocationRoots {

	if !m.MatchesRawInvocation(r) || !r.Frame.executing || m.closed || m.SourceFinalizing || m.constructionFailed {
		return nil
	}

	if c := r.Frame.rationalSetContext; c.mpfRationalSet != nil && c.rationalSetFrame(m, true) == nil {
		return nil
	}
	if c := r.Frame.conversionContext; c.mpfConversion != nil && c.conversionFrame(m, true) == nil {
		return nil
	}
	if c := r.Frame.formatContext; c.mpfFormat != nil && c.formatFrame(m, true) == nil {
		return nil
	}

	if r.Frame.caller != nil && r.Frame.caller.callRoots(m, nil) == nil {
		return nil
	}
	return &r.Frame.Numeric
}
func (m *IntegerManager) beginRawInvocation(s *IntegerRawScope, count uint8) (*integerRawInvocationReceipt, error) {
	if !m.rawFrameAddress(s) || m.closed || m.SourceFinalizing || m.constructionFailed {
		return nil, ErrNumericOwner
	}
	if s.Active || s.Invocation == ^uint64(0) {
		return nil, ErrNumericPending
	}
	*s = IntegerRawScope{owner: m, Invocation: s.Invocation + 1, Active: true, executing: true, Count: count}
	s.receipt = integerRawInvocationReceipt{m, s, s.Invocation}
	return &s.receipt, nil
}
func (m *IntegerManager) finishRawSource(s *IntegerRawScope) error {
	if !m.rawFrameAddress(s) || s.owner != m {
		return ErrNumericOwner
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if e := m.AddMulInvocationSourceCause(&s.AddMul); e != nil {
		s.sourceCause = e
		return e
	}

	if !s.Active || !s.BodySucceeded || s.ended || s.owned {
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
	if !s.pending(m) {
		s.Active = false
	}
	return nil
}
func (m *IntegerManager) RawInvocationSourceCause(r *integerRawInvocationReceipt) error {
	if r != nil && r.Manager == m && m.rawFrameAddress(r.Frame) && r.Invocation != 0 && r.Invocation == r.Frame.Invocation {
		return r.Frame.sourceCause
	}
	return nil
}
func (s *IntegerRawScope) pending(m *IntegerManager) bool {
	if m.MatchesAddMulInvocation(&s.AddMul) || NumericAllocationPending(&s.Numeric) {
		return true
	}

	return false
}
func (m *IntegerManager) CleanupRawInvocation(r *integerRawInvocationReceipt, abort bool) error {
	if !m.MatchesRawInvocation(r) {
		return nil
	}
	s := r.Frame
	if s.executing {
		return ErrNumericPending
	}
	if e := m.CleanupAddMulInvocation(&s.AddMul, false); e != nil {
		return e
	}

	if (s.BodySucceeded || abort) && !s.owned && !s.ended {
		for s.cursor < s.Count {
			if e := m.CleanupRawNumeral(&s.Values[s.cursor]); e != nil {
				return e
			}
			s.cursor++
		}
		s.owned = true
	}
	if !abort {
		return nil
	}
	if e := m.CleanupAddMulInvocation(&s.AddMul, true); e != nil {
		return e
	}

	if e := s.Numeric.Cleanup(m); e != nil {
		return e
	}
	*s = IntegerRawScope{owner: m, Invocation: s.Invocation, BodySucceeded: s.BodySucceeded, ended: s.ended, owned: s.owned, Count: s.Count, cursor: s.cursor, BodyCause: s.BodyCause, sourceCause: s.sourceCause}
	return nil
}

// Explicit owning acknowledgment; neither method replays a source end DEL.
func (m *IntegerManager) CleanupDivision() error {
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	s := &m.DivisionRaw
	r := integerRawInvocationReceipt{m, s, s.Invocation}
	return m.CleanupRawInvocation(&r, true)
}
func (m *IntegerManager) CleanupExtendedGCD() error {
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	s := &m.ExtendedRaw
	r := integerRawInvocationReceipt{m, s, s.Invocation}
	return m.CleanupRawInvocation(&r, true)
}

// Owning continuation of exactly named RAW fields after source manager destruction.
func (m *IntegerManager) CleanupRawNumeral(p *Int) error {
	if m.SourceFinalized || m.ConstructorUnwound {
		if p.Cell != 0 && !m.IsFailureRef(p.Cell) {
			if e := m.freeAfterSourceFinalize(p.Cell); e != nil {
				return e
			}
		}
		p.Cell = 0
		return nil
	}
	if m.SourceFinalizing {
		if p.Cell != 0 && !m.IsFailureRef(p.Cell) {
			if e := m.free(p.Cell); e != nil {
				return e
			}
		}
		p.Cell = 0
		return nil
	}
	return m.Del(p)
}

// CleanupGCDRaw acknowledges the actual retained Lehmer RAW fields; it does
// not repair source scratch holes or replay source explicit DEL.
func (m *IntegerManager) CleanupGCDRaw() error {
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	s := &m.GcdRaw
	r := integerRawInvocationReceipt{m, s, s.Invocation}
	return m.CleanupRawInvocation(&r, true)
}

// Source: src/util/mpz.cpp:1151-1183.
func (m *IntegerManager) CleanupBitwiseNot() error {
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	s := &m.BitwiseNotRaw
	r := integerRawInvocationReceipt{m, s, s.Invocation}
	return m.CleanupRawInvocation(&r, true)
}

func (m *IntegerManager) CleanupBitwiseOr() error {
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	s := &m.BitwiseOrRaw
	r := integerRawInvocationReceipt{m, s, s.Invocation}
	return m.CleanupRawInvocation(&r, true)
}

func (m *IntegerManager) CleanupBitwiseAnd() error {
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	s := &m.BitwiseAndRaw
	r := integerRawInvocationReceipt{m, s, s.Invocation}
	return m.CleanupRawInvocation(&r, true)
}

func (m *IntegerManager) CleanupBitwiseXor() error {
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	s := &m.BitwiseXorRaw
	r := integerRawInvocationReceipt{m, s, s.Invocation}
	return m.CleanupRawInvocation(&r, true)
}
