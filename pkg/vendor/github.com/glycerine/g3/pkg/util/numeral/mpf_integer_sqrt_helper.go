package numeral

// Source: src/util/mpf.cpp:951-1003.

import

// MyMPZSqrt preserves the original helper's normalized-radicand bounds and
// bisection. An inexact result chooses the upper endpoint as a sticky bit.
umemory "github.com/glycerine/g3/pkg/util/memory"

func MyMPZSqrt(m *IntegerManager, sbits uint32, oddExp bool, input, out *Int) (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.mySqrt
	if s.active || s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	if e := m.check(input); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	*s = integerMPFSqrtHelperScope{manager: m, invocation: s.invocation + 1, active: true, executing: true, live: [5]bool{true, true, true, true, true}}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishMyMPZSqrtSource())
		cause = JoinNumericErrors(cause, s.unbind())
	}()
	lower, upper, mid, product, diff := &s.values[0], &s.values[1], &s.values[2], &s.values[3], &s.values[4]
	context := s.context()
	if e := m.SetInt32(lower, 1); e != nil {
		return e
	}
	if e := m.check(lower); e != nil {
		return e
	}
	if e := m.mul2kCoreWithAllocationContext(lower, sbits+1, context); e != nil {
		return e
	}
	shift := uint32(3)
	if oddExp {
		shift = 4
	}
	if e := m.shiftWithAllocationContext(input, shift, upper, true, context); e != nil {
		return e
	}
	equal, e := m.Equal(lower, upper)
	if e != nil {
		return e
	}
	if equal {
		if e := s.copyValue(out, lower); e != nil {
			return e
		}
	}
	for {
		equal, e := m.Equal(lower, upper)
		if e != nil {
			return e
		}
		if equal {
			return nil
		}
		m.traceMPFSqrtBounds(input, lower, upper)
		if e := m.addSubWithAllocationContext(upper, lower, diff, true, context); e != nil {
			return e
		}
		one, e := m.IsOne(diff)
		if e != nil {
			return e
		}
		if one {
			if e := m.mulWithAllocationContext(lower, lower, product, context); e != nil {
				return e
			}
			exact, e := m.Equal(product, input)
			if e != nil {
				return e
			}
			if exact {
				m.traceMPFSqrtChoice(true)
				return s.copyValue(out, lower)
			}
			m.traceMPFSqrtChoice(false)
			return s.copyValue(out, upper)
		}
		if e := m.addSubWithAllocationContext(lower, upper, mid, false, context); e != nil {
			return e
		}
		if e := m.MachineDiv2kInPlace(mid, 1); e != nil {
			return e
		}
		if e := m.mulWithAllocationContext(mid, mid, product, context); e != nil {
			return e
		}
		m.traceMPFSqrtMid(mid, product)
		less, e := m.Less(product, input)
		if e != nil {
			return e
		}
		if less {
			if e := s.copyValue(lower, mid); e != nil {
				return e
			}
			continue
		}
		greater, e := m.Greater(product, input)
		if e != nil {
			return e
		}
		if greater {
			if e := s.copyValue(upper, mid); e != nil {
				return e
			}
			continue
		}
		enabled, e := mpfSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled {
			equal, e := m.Equal(product, input)
			if e != nil {
				return e
			}
			if e := mpfSourceAssertion(equal, 998, "m.eq(product, in)"); e != nil {
				return e
			}
		}
		return s.copyValue(out, mid)
	}
}

func (m *IntegerManager) myMPZSqrtTransportActive() bool {
	return m != nil && m.self == m && m.mySqrt.valid() && m.mySqrt.NumericExecuting() && m.Memory != nil && m.Memory.FailureTransportActive()
}

func (s *integerMPFSqrtHelperScope) copyValue(dst, src *Int) error {
	return s.manager.copyFromOwnerWithAllocationContext(dst, s.manager, src, &s.copy, s.context())
}
