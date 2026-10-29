package numeral

// Source: src/util/mpq.cpp:156-185; src/util/mpq.h:269.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// DisplayDecimalInto borrows the caller stream and follows the original
// unrounded Div/display/Rem loop. CleanupDecimalStream joins a failed body.
func (m *RationalManager) DisplayDecimalInto(out RationalSourceStream, a *Rational, precision uint32, truncate bool) (cause error) {
	return m.DisplayDecimalIntoWithInvocation(out, a, precision, truncate, nil)
}

func (m *RationalManager) DisplayDecimalIntoWithInvocation(out RationalSourceStream, a *Rational, precision uint32, truncate bool, receipt *RationalDecimalInvocationReceipt) (cause error) {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	s, e := m.beginDecimalStreamWithInvocation(out, receipt)
	if e != nil {
		return e
	}
	defer func() { cause = s.finishCall(cause) }()
	return s.run(out, a, precision, truncate)
}

func (m *RationalManager) beginDecimalStreamWithInvocation(out RationalSourceStream, receipt *RationalDecimalInvocationReceipt) (*rationalDecimalStreamScope, error) {
	s := &m.decimalStream
	if s.active {
		return nil, ErrNumericPending
	}
	if out == nil {
		return nil, utext.ErrOwnedStringOwner
	}
	if s.invocation == ^uint64(0) {
		return nil, ErrNumericPending
	}
	// Literal n1,d1,v1 declarations precede the first component access.
	*s = rationalDecimalStreamScope{manager: m, invocation: s.invocation + 1, count: 3, active: true, executing: true}
	if receipt != nil {
		*receipt = RationalDecimalInvocationReceipt{m, s.invocation}
	}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return nil, e
	}
	return s, nil
}
func (s *rationalDecimalStreamScope) finishCall(cause error) error {
	s.executing = false
	s.bodyCause = cause
	s.bodySucceeded = cause == nil
	cause = JoinNumericErrors(cause, s.manager.finishDecimalStreamSource())
	cause = JoinNumericErrors(cause, s.unbind())
	if cause == nil {
		s.active = false
	}
	return cause
}
func (s *rationalDecimalStreamScope) run(out RationalSourceStream, a *Rational, precision uint32, truncate bool) error {
	z, c := s.manager.Z, integerAllocationInvocationContext{Caller: &s.hooks}
	if e := z.copyFromOwnerWithAllocationContext(&s.values[1], z, &a.Num, nil, c); e != nil {
		return e
	}
	if e := z.copyFromOwnerWithAllocationContext(&s.values[2], z, &a.Den, nil, c); e != nil {
		return e
	}
	return s.runCopied(out, a.Num.Val < 0, precision, truncate)
}
func (s *rationalDecimalStreamScope) runCopied(out RationalSourceStream, negative bool, precision uint32, truncate bool) error {
	z, c := s.manager.Z, integerAllocationInvocationContext{Caller: &s.hooks}
	ten, n, d, v := &s.values[0], &s.values[1], &s.values[2], &s.values[3]
	if negative {
		if e := out.AppendCString("-"); e != nil {
			return e
		}
		if e := z.negWithAllocationContext(n, c); e != nil {
			return e
		}
	}
	*ten = NewInt(10)
	s.count = 4
	if e := z.divWithAllocationContext(n, d, v, c); e != nil {
		return e
	}
	if e := z.insertSourceIntegerTracked(out, v, nil, &s.insertion); e != nil {
		return e
	}
	if e := z.remWithAllocationContext(n, d, n, c); e != nil {
		return e
	}
	if n.Val == 0 {
		return nil
	}
	if e := out.AppendCString("."); e != nil {
		return e
	}
	for i := uint32(0); i < precision; i++ {
		if e := z.mulWithAllocationContext(n, ten, n, c); e != nil {
			return e
		}
		if e := z.divWithAllocationContext(n, d, v, c); e != nil {
			return e
		}
		if modularSourceDebug {
			dbg := diagnostic.BorrowProcessDebugContext()
			enabled, e := diagnostic.SourceAssertionsEnabled(dbg)
			if e != nil {
				return e
			}
			if enabled {
				less, e := z.Less(v, ten)
				if e != nil {
					return e
				}
				if !less {
					if e := diagnostic.DebugAssertionFailure(dbg, "src/util/mpq.cpp", 174, "lt(v1, ten)"); e != nil {
						return e
					}
				}
			}
		}
		if e := z.insertSourceIntegerTracked(out, v, nil, &s.insertion); e != nil {
			return e
		}
		if e := z.remWithAllocationContext(n, d, n, c); e != nil {
			return e
		}
		if n.Val == 0 {
			return nil
		}
	}
	if !truncate {
		return out.AppendCString("?")
	}
	return nil
}

type rationalDecimalStreamScope struct {
	manager                        *RationalManager
	invocation                     uint64
	values                         [4]Int // ten,n1,d1,v1: literal source end-label order
	count, sourceCursor, ownCursor uint8
	hooks                          NumericInvocationHooks
	numeric                        NumericAllocationRoots
	division                       integerRawInvocationReceipt

	insertion                                   NumeralInsertionReceipt
	integerBinding, naturalBinding              NumericCallerBinding
	active, executing, finishing, bodySucceeded bool
	bodyCause, sourceCause, ownCause            error
}

func (s *rationalDecimalStreamScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.decimalStream && s.invocation != 0
}
func (s *rationalDecimalStreamScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *rationalDecimalStreamScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && (s.manager.Z.Memory == nil || !s.manager.Z.Memory.FailureTransportActive()))
}
func (s *rationalDecimalStreamScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager.Z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}
func (s *rationalDecimalStreamScope) NumericRecord(z *IntegerManager, event string, r NumericInvocationReceipt) {
	if s.NumericBound(true) && z == s.manager.Z && event == "division" && r.Raw != nil {
		s.division = *r.Raw
	}
}

func (s *rationalDecimalStreamScope) bind() error {
	z := s.manager.Z
	if e := z.BindCaller(&s.integerBinding, s, 33); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 33); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *rationalDecimalStreamScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (m *RationalManager) finishDecimalStreamSource() error {
	s := &m.decimalStream
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
	for _, e := range []error{s.insertion.SourceCause(), m.Z.RawInvocationSourceCause(&s.division)} {
		if e != nil {
			s.sourceCause = e
			return e
		}
	}
	if !s.bodySucceeded {
		return nil
	} // Exception skips the literal end label.
	for s.sourceCursor < 4 {
		if _, e := SourceNumeralDel(m.Z, &s.values[s.sourceCursor]); e != nil {
			s.sourceCause = e
			return e
		}
		s.sourceCursor++
	}
	return nil
}
func (m *RationalManager) CleanupDecimalStream() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.decimalStream
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
	defer func() {
		s.finishing = false
		cause = JoinNumericErrors(cause, s.unbind())
		if cause != nil && s.ownCause == nil {
			s.ownCause = cause
		}
	}()
	z := m.Z
	if s.insertion.active {
		if e := cleanupOwnedNumeralInsertion(&s.insertion, z.Natural); e != nil {
			return e
		}
	}

	if e := z.CleanupRawInvocation(&s.division, true); e != nil {
		return e
	}
	for s.ownCursor < 4 {
		if s.ownCursor != 0 || s.count >= 4 {
			if e := z.CleanupRawNumeral(&s.values[s.ownCursor]); e != nil {
				return e
			}
		}
		s.ownCursor++
	}

	if e := s.numeric.Cleanup(z); e != nil {
		return e
	}
	s.active = false
	return nil
}
func (z *IntegerManager) decimalStreamExecuting(transport bool) bool {
	if z == nil || z.self != z {
		return false
	}
	if transport && (z.Memory == nil || !z.Memory.FailureTransportActive()) {
		return false
	}
	for b := z.callerBindings; b != nil; b = b.previous {
		s, ok := b.guard.(*rationalDecimalStreamScope)
		if b.self == b && ok && s.NumericExecuting() && s.manager.Z == z {
			return true
		}
	}
	return false
}
