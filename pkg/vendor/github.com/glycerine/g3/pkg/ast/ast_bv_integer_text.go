package ast

// Source: src/util/mpq.cpp:123-126; src/util/mpz.cpp:1432-1436.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Actual integer branch's lexical ostringstream and returned std::string.
// Storage is embedded in the caller frame. This is not the diagnostic String
// API and does not assemble or clone the numerical value before insertion.
type astBVIntegerText struct {
	self                                                 *astBVIntegerText
	service                                              *unumeral.RationalService
	stream                                               *utext.NumericTextStream
	result                                               utext.OwnedString
	invocation                                           unumeral.RationalServiceInvocation
	returned, streamEnded, sourceDone, ownFalse, ownTrue bool
	bodyCause, sourceCause, ownCause                     error
}

func (x *astBVIntegerText) construct(q *unumeral.RationalService, b *arena.Budget, value *unumeral.OwnedRational) (e error) {
	if x == nil || x.self != nil {
		return ErrASTHandle
	}
	x.self = x
	x.service = q
	defer func() { x.bodyCause = e }()
	integer, e := q.IsInt(value)
	if e != nil {
		return e
	}
	if !integer {
		return unumeral.ErrRationalNotInteger
	}
	x.stream, e = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b})
	if e != nil {
		return e
	}
	e = q.InsertIntoWithInvocation(x.stream, value, &x.invocation)
	var stop *unumeral.NumeralSourceFailure
	if errors.As(e, &stop) {
		x.sourceCause = stop.DestructorCause
		return e
	}
	if e == nil {
		e = utext.CaptureNumericStreamOwnedString(&x.result, x.stream, b)
		if e == nil {
			x.returned = true
		}
	}
	// The actual return object is constructed before the stream automatic DT.
	close := x.stream.Close()
	if close == nil {
		x.streamEnded = true
	} else if x.sourceCause == nil {
		x.sourceCause = close
	}
	return errors.Join(e, close)
}
func (x *astBVIntegerText) sourceDestroy() error {
	if x == nil || x.self != x {
		return ErrASTHandle
	}
	if e := x.invocation.SourceReady(); e != nil {
		return e
	}
	if x.sourceCause != nil {
		return x.sourceCause
	}
	if x.sourceDone {
		return nil
	}
	if x.stream != nil && !x.streamEnded {
		if e := x.stream.Close(); e != nil {
			x.sourceCause = e
			return e
		}
		x.streamEnded = true
	}
	if x.returned {
		x.result.SourceDestroy()
	}
	x.sourceDone = true
	return nil
}
func (x *astBVIntegerText) cleanup(abort bool) error {
	if x == nil || x.self != x {
		return ErrASTHandle
	}
	stop := func(e error) error {
		if e != nil && x.ownCause == nil {
			x.ownCause = e
		}
		return e
	}
	if !abort {
		if x.ownFalse {
			return nil
		}
		if e := x.invocation.Cleanup(false); e != nil {
			return stop(e)
		}
		if x.stream != nil && !x.streamEnded {
			if e := x.stream.Close(); e != nil {
				return stop(e)
			}
			x.streamEnded = true
		}
		if x.returned {
			x.result.SourceDestroy()
		}
		x.ownFalse = true
		return nil
	}
	if !x.ownFalse {
		return ErrASTPending
	}
	if x.ownTrue {
		return nil
	}
	x.result.OwnAbort()
	if e := x.invocation.Cleanup(true); e != nil {
		return stop(e)
	}
	if e := x.invocation.EndContext(); e != nil {
		return stop(e)
	}
	x.ownTrue = true
	return nil
}

type astBVIntegerCString struct {
	owner      *astBVIntegerText
	generation uint64
}

func (v astBVIntegerCString) Byte(i uint64) (byte, error) {
	x := v.owner
	if x == nil || x.self != x || !x.returned || x.result.Closed || !x.result.Ready || v.generation != x.result.Generation {
		return 0, ErrASTHandle
	}
	if i > x.result.Length {
		return 0, arena.ErrBounds
	}
	return x.result.CStringByte(i)
}
func (v astBVIntegerCString) Length() (uint64, error) {
	for i := uint64(0); ; i++ {
		b, e := v.Byte(i)
		if e != nil {
			return 0, e
		}
		if b == 0 {
			return i, nil
		}
	}
}
func (x *astBVIntegerText) cstring() (unumeral.ArbitraryFloatFormattedCString, error) {
	if x == nil || x.self != x || !x.returned || x.result.Closed || !x.result.Ready {
		return unumeral.ArbitraryFloatFormattedCString{}, ErrASTHandle
	}
	return unumeral.BorrowArbitraryFloatFormattedCString(astBVIntegerCString{x, x.result.Generation}), nil
}

func (x *astBVIntegerText) storageBytes() uint64 {
	n := x.result.StorageBytes()
	if x.stream != nil {
		s := x.stream.Stats()
		n += s.OwnerBytes + s.ScratchBytes + s.OutputOwnerBytes + s.OutputOverflowBytes
	}
	return n
}
