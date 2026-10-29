package numeral

// Source: src/util/mpz.cpp:1412-1429; src/util/mpq.cpp:142-153.

import (

	// DisplaySMT2Into borrows out for the original sequence of insertions. Any
	// emitted prefix belongs to the caller; CleanupSMT2Stream joins failed locals.
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *IntegerManager) DisplaySMT2Into(out RationalSourceStream, a *Int, decimal bool) (cause error) {
	if e := m.check(a); e != nil {
		return e
	}
	s := &m.smt2Stream
	if s.active {
		return ErrNumericPending
	}
	if out == nil {
		return utext.ErrOwnedStringOwner
	}
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = integerSMT2StreamScope{manager: m, invocation: s.invocation + 1, active: true, executing: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishSMT2StreamSource())
		cause = JoinNumericErrors(cause, s.unbind())
		if cause == nil {
			s.active = false
		}
	}()
	input := a
	if a.Val < 0 {
		s.tmpLive = true // The DEFAULT scoped numeral precedes its SET.
		if e := m.copyFromOwnerWithAllocationContext(&s.tmp, m, a, &s.copy, integerAllocationInvocationContext{}); e != nil {
			return e
		}
		if e := m.negWithAllocationContext(&s.tmp, integerAllocationInvocationContext{Caller: &s.hooks}); e != nil {
			return e
		}
		input = &s.tmp
		if e := out.AppendCString("(- "); e != nil {
			return e
		}
	}
	if e := m.insertSourceIntegerTracked(out, input, nil, &s.insertion); e != nil {
		return e
	}
	if decimal {
		if e := out.AppendCString(".0"); e != nil {
			return e
		}
	}
	if s.tmpLive {
		return out.AppendCString(")")
	}
	return nil
}

type integerSMT2StreamScope struct {
	manager                        *IntegerManager
	invocation                     uint64
	tmp                            Int
	tmpLive                        bool
	copy                           integerCopyAllocationReceipt
	hooks                          NumericInvocationHooks
	numeric                        NumericAllocationRoots
	insertion                      NumeralInsertionReceipt
	integerBinding, naturalBinding NumericCallerBinding
	active, executing, finishing   bool
	bodyCause, sourceCause         error
}

func (s *integerSMT2StreamScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.smt2Stream && s.invocation != 0
}
func (s *integerSMT2StreamScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *integerSMT2StreamScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && (s.manager.Memory == nil || !s.manager.Memory.FailureTransportActive()))
}
func (s *integerSMT2StreamScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}
func (s *integerSMT2StreamScope) bind() error {
	if e := s.manager.BindCaller(&s.integerBinding, s, 30); e != nil {
		return e
	}
	if e := s.manager.Natural.BindCaller(&s.naturalBinding, s, 30); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *integerSMT2StreamScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (m *IntegerManager) finishSMT2StreamSource() error {
	s := &m.smt2Stream
	if s.executing || s.finishing {
		return ErrNumericPending
	}
	if !s.active {
		return nil
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if e := s.insertion.SourceCause(); e != nil {
		s.sourceCause = e
		return e
	}
	if s.tmpLive {
		if _, e := SourceNumeralDel(m, &s.tmp); e != nil {
			s.sourceCause = e
			return e
		}
		s.tmpLive = false
	}
	return nil
}
func (m *IntegerManager) CleanupSMT2Stream() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.smt2Stream
	if s.executing || s.finishing || m.Memory != nil && m.Memory.FailureTransportActive() {
		return ErrNumericPending
	}
	if !s.active {
		return nil
	}
	if !s.valid() {
		return ErrNumericOwner
	}
	if e := s.bind(); e != nil {
		return e
	}
	s.finishing = true
	defer func() { s.finishing = false; cause = JoinNumericErrors(cause, s.unbind()) }()
	if s.insertion.active {
		if e := cleanupOwnedNumeralInsertion(&s.insertion, m.Natural); e != nil {
			return e
		}
	}
	if s.tmpLive {
		if e := m.CleanupRawNumeral(&s.tmp); e != nil {
			return e
		}
		s.tmpLive = false
	}
	if e := m.CleanupCopyAllocationInvocation(&s.copy); e != nil {
		return e
	}
	if e := s.numeric.Cleanup(m); e != nil {
		return e
	}
	s.active = false
	return nil
}

// DisplaySMT2Into preserves the source integer test and separate numerator and
// denominator calls. It stores no ownership of the borrowed stream or input.
func (m *RationalManager) DisplaySMT2Into(out RationalSourceStream, a *Rational, decimal bool) (cause error) {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	s := &m.smt2Stream
	if s.active {
		return ErrNumericPending
	}
	if out == nil {
		return utext.ErrOwnedStringOwner
	}
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = rationalSMT2StreamScope{manager: m, invocation: s.invocation + 1, active: true, executing: true}
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishSMT2StreamSource())
		cause = JoinNumericErrors(cause, s.unbind())
		if cause == nil {
			s.active = false
		}
	}()
	integer, e := m.IsInt(a)
	if e != nil {
		return e
	}
	if integer {
		return s.insert(out, &a.Num, decimal)
	}
	if e := out.AppendCString("(/ "); e != nil {
		return e
	}
	if e := s.insert(out, &a.Num, decimal); e != nil {
		return e
	}
	if e := out.AppendCString(" "); e != nil {
		return e
	}
	if e := s.insert(out, &a.Den, decimal); e != nil {
		return e
	}
	return out.AppendCString(")")
}

type rationalSMT2StreamScope struct {
	manager                        *RationalManager
	invocation, child              uint64
	childOwned                     bool
	integerBinding, naturalBinding NumericCallerBinding
	active, executing, finishing   bool
	bodyCause, sourceCause         error
}

func (s *rationalSMT2StreamScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.smt2Stream && s.invocation != 0
}
func (s *rationalSMT2StreamScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *rationalSMT2StreamScope) bind() error {
	z := s.manager.Z
	if e := z.BindCaller(&s.integerBinding, s, 31); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 31); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *rationalSMT2StreamScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (s *rationalSMT2StreamScope) insert(out RationalSourceStream, a *Int, decimal bool) error {
	z := s.manager.Z
	before := z.smt2Stream.invocation
	e := z.DisplaySMT2Into(out, a, decimal)
	if z.smt2Stream.invocation != before {
		s.child = z.smt2Stream.invocation
		s.childOwned = false
	}
	if e != nil {
		return e
	}
	if s.child != 0 && !s.childOwned {
		if e := z.CleanupSMT2Stream(); e != nil {
			return e
		}
		s.childOwned = true
	}
	return nil
}
func (m *RationalManager) finishSMT2StreamSource() error {
	s := &m.smt2Stream
	if s.executing || s.finishing {
		return ErrNumericPending
	}
	if !s.active {
		return nil
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if s.child != 0 && !s.childOwned {
		if m.Z.smt2Stream.invocation != s.child {
			return ErrNumericOwner
		}
		s.sourceCause = m.Z.smt2Stream.sourceCause
	}
	return s.sourceCause
}
func (m *RationalManager) CleanupSMT2Stream() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.smt2Stream
	if s.executing || s.finishing || m.Z.Memory != nil && m.Z.Memory.FailureTransportActive() {
		return ErrNumericPending
	}
	if !s.active {
		return nil
	}
	if !s.valid() {
		return ErrNumericOwner
	}
	if e := s.bind(); e != nil {
		return e
	}
	s.finishing = true
	defer func() { s.finishing = false; cause = JoinNumericErrors(cause, s.unbind()) }()
	if s.child != 0 && !s.childOwned {
		if m.Z.smt2Stream.invocation != s.child {
			return ErrNumericOwner
		}
		if e := m.Z.CleanupSMT2Stream(); e != nil {
			return e
		}
		s.childOwned = true
	}
	s.active = false
	return nil
}

// Only these actual borrowed namespaces are blocked during runtime transport.
func (z *IntegerManager) smt2StreamExecuting(transport bool) bool {
	if z == nil || z.self != z {
		return false
	}
	if transport && (z.Memory == nil || !z.Memory.FailureTransportActive()) {
		return false
	}
	for b := z.callerBindings; b != nil; b = b.previous {
		if b.self != b {
			continue
		}
		switch s := b.guard.(type) {
		case *integerSMT2StreamScope:
			if s.NumericExecuting() && s.manager == z {
				return true
			}
		case *rationalSMT2StreamScope:
			if s.NumericExecuting() && s.manager.Z == z {
				return true
			}
		}
	}
	return false
}
