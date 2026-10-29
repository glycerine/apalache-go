package numeral

// Source: src/util/mpbq.cpp:394-401; scoped_numeral.h:186-189.

import (
	"github.com/glycerine/g3/pkg/arena"
	"strings"
)

// BinaryRationalSourceStream exposes the two reached ostream insertion types.
type BinaryRationalSourceStream interface {
	AppendCString(string) error
	AppendUint32(uint32) error
}

func (m *BinaryRationalManager) DisplaySourceTracked(out BinaryRationalSourceStream, a *BinaryRational, receipt *NumeralInsertionReceipt) error {
	if out == nil {
		return arena.ErrInvalidSlot
	}
	if e := m.check(a); e != nil {
		return e
	}
	var text strings.Builder
	if e := m.Z.InsertSourceIntegerTracked(integerSourceStringStream{&text}, a.BorrowNumerator(), receipt, nil); e != nil {
		return e
	}
	if e := out.AppendCString(text.String()); e != nil {
		return e
	}
	if a.KField > 0 {
		if e := out.AppendCString("/2"); e != nil {
			return e
		}
	}
	if a.KField > 1 {
		if e := out.AppendCString("^"); e != nil {
			return e
		}
		return out.AppendUint32(a.KField)
	}
	return nil
}
