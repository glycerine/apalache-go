package numeral

// Source: src/util/mpq.cpp:123-127; src/util/mpq.h:255; src/util/mpz.cpp:1432-1435.

import (

	// ToOwnedString follows the selected left-to-right numerator/slash/denominator
	// string sequence. Success transfers the fresh return slot to the caller.
	// Failure retains actual temporaries for CleanupOwnedString or CloseChecked.
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *RationalManager) ToOwnedString(a *Rational, out *utext.OwnedString) (cause error) {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	if m.stringScope.active {
		return ErrNumericPending
	}
	if out == nil || out.Self != nil {
		return utext.ErrOwnedStringOwner
	}
	s := &m.stringScope
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	next := s.invocation + 1
	*s = rationalOwnedStringScope{manager: m, invocation: next, out: out, active: true, executing: true}
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishOwnedStringSource())
		cause = JoinNumericErrors(cause, s.unbind())
		if cause == nil {
			s.out = nil
			s.active = false
		}
	}()
	integer, e := m.IsInt(a)
	if e != nil {
		return e
	}
	if e := s.numeratorStream.Construct(m.BudgetField); e != nil {
		return e
	}
	s.numeratorStreamLive = true
	if e := m.Z.insertSourceIntegerTracked(&s.numeratorStream, &a.Num, nil, &s.numeratorInsertion); e != nil {
		return e
	}
	if e := s.numeratorStream.Str(out); e != nil {
		return e
	}
	s.outLive = true
	s.numeratorStream.Buffer.String.SourceDestroy()
	s.numeratorStreamLive = false
	if integer {
		return nil
	}
	if e := out.AppendCString("/"); e != nil {
		return e
	}
	if e := s.denominatorStream.Construct(m.BudgetField); e != nil {
		return e
	}
	s.denominatorStreamLive = true
	if e := m.Z.insertSourceIntegerTracked(&s.denominatorStream, &a.Den, nil, &s.denominatorInsertion); e != nil {
		return e
	}
	if e := s.denominatorStream.Str(&s.denominator); e != nil {
		return e
	}
	s.denominatorLive = true
	s.denominatorStream.Buffer.String.SourceDestroy()
	s.denominatorStreamLive = false
	if e := out.Append(s.denominator.View()); e != nil {
		return e
	}
	s.denominator.SourceDestroy()
	s.denominatorLive = false
	return nil
}

type rationalOwnedStringScope struct {
	invocation                                                           uint64
	manager                                                              *RationalManager
	out                                                                  *utext.OwnedString
	denominator                                                          utext.OwnedString
	numeratorStream, denominatorStream                                   utext.OwnedStringStream
	numeratorInsertion, denominatorInsertion                             NumeralInsertionReceipt
	integerBinding, naturalBinding                                       NumericCallerBinding
	outLive, denominatorLive, numeratorStreamLive, denominatorStreamLive bool
	active, executing, finishing                                         bool
	bodyCause, sourceCause                                               error
}

func (s *rationalOwnedStringScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.stringScope
}
func (s *rationalOwnedStringScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *rationalOwnedStringScope) bind() error {
	z := s.manager.Z
	if e := z.BindCaller(&s.integerBinding, s, 28); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 28); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *rationalOwnedStringScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (z *IntegerManager) rationalStringExecuting(transport bool) bool {
	if z == nil || z.self != z {
		return false
	}
	for b := z.callerBindings; b != nil; b = b.previous {
		s, ok := b.guard.(*rationalOwnedStringScope)
		if b.self == b && ok && s.NumericExecuting() && s.manager.Z == z && (!transport || z.Memory != nil && z.Memory.FailureTransportActive()) {
			return true
		}
	}
	return false
}

func (m *RationalManager) finishOwnedStringSource() error {
	s := &m.stringScope
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
	for _, r := range [...]*NumeralInsertionReceipt{&s.denominatorInsertion, &s.numeratorInsertion} {
		if e := r.SourceCause(); e != nil {
			s.sourceCause = e
			return e
		}
	}
	for _, local := range [...]struct {
		str  *utext.OwnedString
		live *bool
	}{{&s.denominator, &s.denominatorLive}, {&s.denominatorStream.Buffer.String, &s.denominatorStreamLive}, {&s.numeratorStream.Buffer.String, &s.numeratorStreamLive}} {
		if *local.live {
			local.str.SourceDestroy()
			*local.live = false
		}
	}
	if s.bodyCause != nil && s.outLive {
		s.out.SourceDestroy()
		s.outLive = false
	}
	return nil
}

func (m *RationalManager) CleanupOwnedString() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.stringScope
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
	for _, r := range [...]*NumeralInsertionReceipt{&s.denominatorInsertion, &s.numeratorInsertion} {
		if r.Active() {
			if e := cleanupOwnedNumeralInsertion(r, m.Z.Natural); e != nil {
				return e
			}
		}
	}
	s.denominator.OwnAbort()
	s.denominatorStream.OwnAbort()
	s.numeratorStream.OwnAbort()
	if s.out != nil {
		s.out.OwnAbort()
	}
	s.outLive, s.denominatorLive, s.numeratorStreamLive, s.denominatorStreamLive = false, false, false, false
	s.out = nil
	s.active = false
	return nil
}
func (s *rationalOwnedStringScope) storageBytes() uint64 {
	var n uint64
	for _, str := range [...]*utext.OwnedString{&s.denominator, &s.denominatorStream.Buffer.String, &s.numeratorStream.Buffer.String, s.out} {
		if str != nil {
			n += str.StorageBytes()
		}
	}
	return n
}
