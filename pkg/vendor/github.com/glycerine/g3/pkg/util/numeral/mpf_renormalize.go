package numeral

// Source: src/util/mpf.cpp:1255-1270.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	// Renormalize shifts a nonnegative significand into the given precision and
	// adjusts its exponent. Like the source helper, it ignores ebits and truncates
	// discarded bits without rounding. The caller owns both output fields.
)

func (m *ArbitraryFloatManager) Renormalize(ebits, sbits uint32, exponent *int64, sig *Int) (cause error) {
	if m == nil || m.self != m || m.integer == nil || m.integer.self != m.integer {
		return umemory.ErrRuntimeOwner
	}
	if m.renormalize.active {
		return ErrNumericPending
	}
	if e := m.valueOwnerGuard(); e != nil {
		return e
	}
	if exponent == nil {
		return umemory.ErrRuntimeOwner
	}
	if e := m.integer.check(sig); e != nil {
		return e
	}
	s := &m.renormalize
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatRenormalizeScope{manager: m, invocation: s.invocation + 1, active: true, executing: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		s.active = cause != nil || NumericAllocationPending(&s.numeric)
		cause = JoinNumericErrors(cause, s.unbind())
	}()
	return m.renormalizeWithContext(ebits, sbits, exponent, sig, s.context())
}

func (m *ArbitraryFloatManager) renormalizeWithContext(ebits, sbits uint32, exponent *int64, sig *Int, context integerAllocationInvocationContext) error {
	z := m.integer
	if context.Caller != nil && !context.Caller.callBound(true) {
		return ErrNumericOwner
	}
	zero, e := z.IsZero(sig)
	if e != nil {
		return e
	}
	if zero {
		return nil
	}
	w, e := m.powerTwoWrapper(sbits, false)
	if e != nil {
		return e
	}
	pg, e := w.Capture()
	if e != nil {
		return e
	}
	for {
		ge, e := z.GreaterEqual(sig, &pg)
		if e != nil {
			return e
		}
		if !ge {
			break
		}
		if e := z.MachineDiv2kInPlace(sig, 1); e != nil {
			return e
		}
		// The original signed increment is undefined at INT64_MAX. Diagnose it
		// after the preceding source significand shift has taken place.
		if *exponent == int64(1<<63-1) {
			return arena.ErrBounds
		}
		*exponent++
	}
	w, e = m.powerTwoWrapper(sbits-1, false)
	if e != nil {
		return e
	}
	pl, e := w.Capture()
	if e != nil {
		return e
	}
	for {
		less, e := z.Less(sig, &pl)
		if e != nil {
			return e
		}
		if !less {
			return nil
		}
		if e := z.mul2kCoreWithAllocationContext(sig, 1, context); e != nil {
			return e
		}
		if *exponent == int64(-1<<63) {
			return arena.ErrBounds
		}
		*exponent--
	}
}
