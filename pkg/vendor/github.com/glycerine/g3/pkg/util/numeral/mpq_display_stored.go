package numeral

// Source: src/util/mpq.cpp:130-139; mpq.h:253; mpz.h:758-760.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// DisplayStoredToTracked borrows the concrete NUM/DEN records of a stored mpq.
// The integer predicate precedes numerator insertion; denominator display reads
// its record again after the slash insertion. No numeral ownership is acquired.
func (m *RationalManager) DisplayStoredToTracked(w io.Writer, allocator *umemory.SmallAllocator, object umemory.ObjectRef, receipt *NumeralInsertionReceipt) error {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if e := m.checkOwnerReady(); e != nil {
		return e
	}
	if allocator == nil {
		return umemory.ErrAllocatorOwner
	}
	lo, e := allocator.ReadUint32(object, 16)
	if e != nil {
		return e
	}
	hi, e := allocator.ReadUint32(object, 20)
	if e != nil {
		return e
	}
	integer := false
	if lo == 0 && hi == 0 {
		value, e := allocator.ReadUint32(object, 12)
		if e != nil {
			return e
		}
		integer = value == 1
	}
	n, e := decimalStoredInteger(allocator, object, 0)
	if e != nil {
		return e
	}
	out := numeralDisplayStream{w}
	if e = m.Z.InsertSourceIntegerTracked(out, &n, receipt, nil); e != nil {
		return e
	}
	if integer {
		return nil
	}
	if e = out.AppendCString("/"); e != nil {
		return e
	}
	d, e := decimalStoredInteger(allocator, object, 12)
	if e != nil {
		return e
	}
	return m.Z.InsertSourceIntegerTracked(out, &d, receipt, nil)
}
