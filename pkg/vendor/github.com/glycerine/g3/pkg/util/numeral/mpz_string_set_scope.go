package numeral

// Source: src/util/mpz.cpp:283-303; src/util/mpz.h:692.

import

// Actual RAW ten/tmp fields and reached end DEL in the charged Integer owner.
// Body failure omits DEL; owning cleanup never advances that source history.
umemory "github.com/glycerine/g3/pkg/util/memory"

type integerStringSetScope struct {
	manager                                                   *IntegerManager
	invocation                                                uint64
	ten, tmp                                                  Int
	declared, endReached, tmpDelDone, tmpOwned, bodySucceeded bool
	index                                                     int
	stage                                                     uint8
	negative                                                  bool
	hooks                                                     NumericInvocationHooks
	numeric                                                   NumericAllocationRoots

	integerBinding, naturalBinding   NumericCallerBinding
	active, executing, finishing     bool
	bodyCause, sourceCause, ownCause error
}

func (s *integerStringSetScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.stringSet && s.invocation != 0
}
func (s *integerStringSetScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *integerStringSetScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && (s.manager.Memory == nil || !s.manager.Memory.FailureTransportActive()))
}
func (s *integerStringSetScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}

func (s *integerStringSetScope) bind() error {
	z := s.manager
	if e := z.BindCaller(&s.integerBinding, s, 38); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 38); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *integerStringSetScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (s *integerStringSetScope) cleanupArithmetic() error {
	z := s.manager

	return s.numeric.Cleanup(z)
}
func (m *IntegerManager) stringSetTransportActive() bool {
	return m != nil && m.stringSet.NumericExecuting() && m.Memory != nil && m.Memory.FailureTransportActive()
}
func (m *IntegerManager) stringSetSourceReady() error {
	s := &m.stringSet
	if !s.active {
		return nil
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	return ErrNumericPending
}

// CleanupStringSet joins only this parser's RAW temporary and reached allocation
// roots. It does not delete the borrowed destination or replay a skipped end label.
func (m *IntegerManager) CleanupStringSet() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.stringSet
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
	defer func() {
		s.finishing = false
		if cause != nil && s.ownCause == nil {
			s.ownCause = cause
		}
		cause = JoinNumericErrors(cause, s.unbind())
	}()

	if s.declared && !s.tmpDelDone && !s.tmpOwned {
		if e := m.CleanupRawNumeral(&s.tmp); e != nil {
			return e
		}
		s.tmpOwned = true
	}
	if e := s.cleanupArithmetic(); e != nil {
		return e
	}
	s.active = false
	return nil
}
