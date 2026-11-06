package numeral

// Source: src/util/mpf.cpp:1519-1569; src/util/mpq.cpp:156-185.
// rational display

import

// ArbitraryFloatFormatStream borrows the reached default-profile stringstream.
// Each call is one source insertion. Source and owning ends remain distinct.
umemory "github.com/glycerine/g3/pkg/util/memory"

type ArbitraryFloatFormatStream interface {
	RationalSourceStream
	AppendInt64(int64) error
	SourceFinalize() error
	Close() error
}

// ArbitraryFloatFormatTarget owns the source res string. Its constructor precedes
// classification; the stream constructor occurs only after rational division.
// AppendStreamString performs the actual ss.str copy, append, and temporary DT.
type ArbitraryFloatFormatTarget interface {
	NumericInvocation() *ArbitraryFloatFormatInvocation
	AssignLiteral(string) error
	NewArbitraryFloatFormatStream() (ArbitraryFloatFormatStream, error)
	AppendStreamString(ArbitraryFloatFormatStream) error
	CString() (ArbitraryFloatFormattedCString, error)
	SourceFinalize() error
	Close() error
}

// ArbitraryFloatFormatCString is a cold borrowed view whose concrete source
// owner validates its captured identity on each reached read.
type ArbitraryFloatFormatCString interface {
	Byte(uint64) (byte, error)
	Length() (uint64, error)
}
type ArbitraryFloatFormattedCString struct{ view ArbitraryFloatFormatCString }

func BorrowArbitraryFloatFormattedCString(v ArbitraryFloatFormatCString) ArbitraryFloatFormattedCString {
	return ArbitraryFloatFormattedCString{view: v}
}
func (v ArbitraryFloatFormattedCString) Byte(i uint64) (byte, error) {
	if v.view == nil {
		return 0, umemory.ErrRuntimeOwner
	}
	return v.view.Byte(i)
}
func (v ArbitraryFloatFormattedCString) Length() (uint64, error) {
	if v.view == nil {
		return 0, umemory.ErrRuntimeOwner
	}
	return v.view.Length()
}
