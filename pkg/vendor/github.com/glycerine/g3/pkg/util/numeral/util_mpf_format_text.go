package numeral

// Source: src/util/mpf.cpp:1519-1569.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// The result and counted output snapshot have independent Go-managed text.
// This used cold transport header is charged once; NUM scope binding follows
// the exact numerical invocation seam rather than owning numerals here.
type ArbitraryFloatFormatText struct {
	Numeric                  ArbitraryFloatFormatInvocation
	Self                     *ArbitraryFloatFormatText
	budget                   *arena.Budget
	Result, Temporary        utext.OwnedString
	Stream                   mpfFormatStream
	Header                   uint64
	closed, SourceEndedField bool
}
type mpfFormatStream struct {
	*utext.NumericTextStream
	owner       *ArbitraryFloatFormatText
	ClosedField bool
}
type mpfFormatCString struct {
	owner      *ArbitraryFloatFormatText
	generation uint64
}

func NewArbitraryFloatFormatText(b *arena.Budget) (*ArbitraryFloatFormatText, error) {
	if b == nil {
		return nil, utext.ErrNumericStreamConfig
	}
	n := uint64(reflect.TypeOf(ArbitraryFloatFormatText{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	t := &ArbitraryFloatFormatText{budget: b, Header: n}
	t.Self = t
	t.Stream.owner = t
	if e := t.Result.Construct(b, ""); e != nil {
		b.Release(n)
		return nil, e
	}
	return t, nil
}
func (t *ArbitraryFloatFormatText) NumericInvocation() *ArbitraryFloatFormatInvocation {
	if t == nil || t.Self != t || t.closed {
		return nil
	}
	return &t.Numeric
}
func (t *ArbitraryFloatFormatText) open() error {
	if t == nil || t.Self != t {
		return utext.ErrNumericStreamOwner
	}
	if t.closed || t.SourceEndedField {
		return arena.ErrClosed
	}
	return nil
}
func (t *ArbitraryFloatFormatText) AssignLiteral(s string) error {
	if e := t.open(); e != nil {
		return e
	}
	return t.Result.Assign(s)
}
func (t *ArbitraryFloatFormatText) NewArbitraryFloatFormatStream() (ArbitraryFloatFormatStream, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	if t.Stream.NumericTextStream != nil {
		return nil, utext.ErrNumericStreamOwner
	}
	s, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.budget})
	if e != nil {
		return nil, e
	}
	t.Stream.NumericTextStream = s
	return &t.Stream, nil
}
func (s *mpfFormatStream) SourceFinalize() error {
	if s == nil || s.owner == nil || s.owner.Self != s.owner || &s.owner.Stream != s {
		return utext.ErrNumericStreamOwner
	}
	if s.ClosedField {
		return nil
	}
	if s.NumericTextStream == nil {
		return utext.ErrNumericStreamOwner
	}
	if e := s.NumericTextStream.Close(); e != nil {
		return e
	}
	s.ClosedField = true
	return nil
}
func (s *mpfFormatStream) Close() error { return s.SourceFinalize() }

// captureStreamTemporary copies counted output, preserving embedded NULs.
func (t *ArbitraryFloatFormatText) captureStreamTemporary(s *utext.NumericTextStream) error {
	return utext.CaptureNumericStreamOwnedString(&t.Temporary, s, t.budget)
}
func (t *ArbitraryFloatFormatText) appendTemporary() error {
	return t.Result.Append(t.Temporary.View())
}
func (t *ArbitraryFloatFormatText) AppendStreamString(stream ArbitraryFloatFormatStream) error {
	if e := t.open(); e != nil {
		return e
	}
	if stream != &t.Stream || t.Stream.ClosedField || t.Stream.NumericTextStream == nil {
		return utext.ErrNumericStreamOwner
	}
	if e := t.captureStreamTemporary(t.Stream.NumericTextStream); e != nil {
		return e
	}
	e := t.appendTemporary()
	// Completed lexical temporary is destroyed even when append throws.
	t.Temporary.SourceDestroy()
	return e
}
func (t *ArbitraryFloatFormatText) CString() (ArbitraryFloatFormattedCString, error) {
	if e := t.open(); e != nil {
		return ArbitraryFloatFormattedCString{}, e
	}
	return BorrowArbitraryFloatFormattedCString(mpfFormatCString{t, t.Result.Generation}), nil
}
func (v mpfFormatCString) Byte(i uint64) (byte, error) {
	if e := v.owner.open(); e != nil {
		return 0, e
	}
	s := &v.owner.Result
	if v.generation != s.Generation {
		return 0, arena.ErrInvalidSlot
	}
	if i > s.Length {
		return 0, arena.ErrBounds
	}
	if i == s.Length {
		return 0, nil
	}
	return s.View().Byte(i)
}
func (v mpfFormatCString) Length() (uint64, error) {
	for i := uint64(0); ; i++ {
		b, e := v.Byte(i)
		if e != nil {
			return 0, e
		}
		if b == 0 {
			return i, nil
		}
		if i == math.MaxUint64 {
			return 0, arena.ErrBounds
		}
	}
}
func (t *ArbitraryFloatFormatText) SourceFinalize() error {
	if t == nil || t.Self != t {
		return utext.ErrNumericStreamOwner
	}
	if t.SourceEndedField {
		return nil
	}
	if e := t.Numeric.SourceReady(); e != nil {
		return e
	}
	if t.Stream.NumericTextStream != nil && !t.Stream.ClosedField {
		return utext.ErrOwnedStringOwner
	}
	t.Result.SourceDestroy()
	t.SourceEndedField = true
	return nil
}
func (t *ArbitraryFloatFormatText) Close() error {
	if t == nil || t.Self != t {
		return utext.ErrNumericStreamOwner
	}
	if t.closed {
		return nil
	}
	if e := t.Numeric.Cleanup(false); e != nil {
		return e
	}
	if e := t.Numeric.Cleanup(true); e != nil {
		return e
	}
	if t.Stream.NumericTextStream != nil {
		if e := t.Stream.Close(); e != nil {
			return e
		}
	}
	t.Temporary.OwnAbort()
	t.Result.OwnAbort()
	if e := t.Numeric.EndContext(); e != nil {
		return e
	}
	t.budget.Release(t.Header)
	t.Header = 0
	t.closed = true
	t.SourceEndedField = true
	return nil
}

// StorageBytes accounts for this concrete transport and its actual storage.
func (t *ArbitraryFloatFormatText) StorageBytes() uint64 {
	n := t.Header + uint64(t.Result.StorageBytes()+t.Temporary.StorageBytes())
	if s := t.Stream.NumericTextStream; s != nil {
		v := s.Stats()
		n += v.OwnerBytes + v.ScratchBytes + v.OutputOwnerBytes + v.OutputOverflowBytes
	}
	return n
}
func (t *ArbitraryFloatFormatText) SourceEnded() bool { return t.SourceEndedField }
