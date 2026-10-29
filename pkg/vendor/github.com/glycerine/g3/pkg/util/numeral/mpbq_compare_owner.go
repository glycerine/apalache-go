package numeral

// Source: src/util/mpbq.cpp::lt; basic_interval.h:118-130. Concrete owner
// views for the retained algebraic arithmetic-plugin translation consumer.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// LessFromOwnerTracked preserves receiver scratch, source numerator ownership,
// and the original copy-then-shift path when exponents differ. The explicit
// receipt owns only an accepted copy; shift growth uses the caller context.
func (o NumericBinaryRationalOperations) LessFromOwnerTracked(source *BinaryRationalManager, a, b *BinaryRational, receipt *IntegerCopyAllocationReceipt) (bool, error) {
	if e := o.valid(); e != nil {
		return false, e
	}
	m := o.manager
	if e := m.open(); e != nil {
		return false, e
	}
	if source == nil {
		return false, arena.ErrConfig
	}
	if e := source.pair(a, b); e != nil {
		return false, e
	}
	if a.KField == b.KField {
		order, e := m.Z.CompareFromOwners(source.Z, &a.Num, source.Z, &b.Num)
		return order < 0 && e == nil, e
	}
	input, delta := &a.Num, b.KField-a.KField
	if a.KField > b.KField {
		input, delta = &b.Num, a.KField-b.KField
	}
	copyContext := o.context
	if receipt != nil {
		copyContext = NumericAllocationContext{}
	}
	if e := m.Z.copyFromOwnerWithAllocationContext(&m.Tmp, source.Z, input, receipt, copyContext); e != nil {
		return false, e
	}
	if e := m.Z.mul2kCoreWithAllocationContext(&m.Tmp, delta, o.context); e != nil {
		return false, e
	}
	var order int
	var e error
	if a.KField < b.KField {
		order, e = m.Z.CompareFromOwners(m.Z, &m.Tmp, source.Z, &b.Num)
	} else {
		order, e = m.Z.CompareFromOwners(source.Z, &a.Num, m.Z, &m.Tmp)
	}
	return order < 0 && e == nil, e
}

// CopyFromOwnerTracked follows the source ordering assertion, lower SET and
// upper SET. It neither normalizes nor stages an extra copy of the interval.
func (o NumericBinaryIntervalOperations) CopyFromOwnerTracked(dst *BinaryInterval, source *BinaryIntervalManager, src *BinaryInterval, receipt *IntegerCopyAllocationReceipt) error {
	if e := o.valid(); e != nil {
		return e
	}
	m := o.manager
	if e := m.check(dst); e != nil {
		return e
	}
	if source == nil || source != m && dst == src {
		return arena.ErrConfig
	}
	if e := source.check(src); e != nil {
		return e
	}
	return o.setBoundsFromOwnerTracked(dst, source.bounds, &src.lower, &src.upper, receipt)
}

// SetBoundsTracked retains the actual interval SET comparison and numeral copies.
func (o NumericBinaryIntervalOperations) SetBoundsTracked(dst *BinaryInterval, lower, upper *BinaryRational, receipt *IntegerCopyAllocationReceipt) error {
	if e := o.valid(); e != nil {
		return e
	}
	if e := o.manager.check(dst); e != nil {
		return e
	}
	if e := o.manager.bounds.pair(lower, upper); e != nil {
		return e
	}
	return o.setBoundsFromOwnerTracked(dst, o.manager.bounds, lower, upper, receipt)
}
func (o NumericBinaryIntervalOperations) setBoundsFromOwnerTracked(dst *BinaryInterval, source *BinaryRationalManager, lower, upper *BinaryRational, receipt *IntegerCopyAllocationReceipt) error {
	m := o.manager
	if SourceExtendedGCDDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled {
			greater, e := m.bounds.WithAllocationContext(o.context).LessFromOwnerTracked(source, upper, lower, receipt)
			if e != nil {
				return e
			}
			if greater {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/basic_interval.h", 119, "m().le(lower, upper)"); e != nil {
					return errors.Join(ErrBinaryInvariant, e)
				}
			}
		}
	}
	if e := m.bounds.CopyFromOwnerWithAllocationReceipt(&dst.lower, source, lower, receipt); e != nil {
		return e
	}
	return m.bounds.CopyFromOwnerWithAllocationReceipt(&dst.upper, source, upper, receipt)
}
