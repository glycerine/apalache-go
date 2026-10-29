package numeral

// Source set/copy semantics, adapted to the explicit numerical owner boundary
// required by GO_PORT_PLAN.md B12. C++ rational uses one global manager instead.

import (
	"github.com/glycerine/g3/pkg/arena"
)

// CopyFrom deep-copies an integer from an explicitly supplied source manager.
// Raw Int has no owner provenance: the caller must supply the actual owner of
// each operand, just as for other low-level IntegerManager methods. The public
// rational service validates provenance before calling this bridge. A single
// raw address cannot simultaneously own a value in two different managers.
// Source storage is only read; destination allocation preserves source live
// size/capacity and sign instead of formatting or normalizing the value.

func (m *IntegerManager) CopyFrom(dst *Int, source *IntegerManager, src *Int) error {
	if m == nil || source == nil {
		return arena.ErrConfig
	}
	if m == source {
		return m.Copy(dst, src)
	}
	if dst == src {
		return arena.ErrConfig
	}
	if e := m.check(dst); e != nil {
		return e
	}
	if e := source.check(src); e != nil {
		return e
	}
	if src.IsSmall() {
		value := src.Val
		if e := m.Del(dst); e != nil {
			return e
		}
		dst.Val = value
		return nil
	}
	cell := source.Cell(src.Cell)
	ref := dst.Cell
	if ref == 0 || m.Cell(ref).Capacity < cell.Size {
		var e error
		ref, e = m.Allocate(cell.Capacity, cell.Size)
		if e != nil {
			return e
		}
	}
	m.ResizeCell(ref, cell.Size)
	// All fallible preparation is complete before changing an existing payload.
	for i := uint32(0); i < cell.Size; i++ {
		m.writeDigit(ref, i, source.Digit(src.Cell, i))
	}
	old := dst.Cell
	dst.Cell, dst.Val = ref, src.Val
	m.PublishCell(ref)
	if old != 0 && old != ref {
		if e := m.free(old); e != nil {
			return e
		}
	}
	return nil
}

// CopyFrom transfers numerical ownership by independent component copies.
// It preserves unreduced fractions, negative denominators and small/big tags.
// The pair is staged in fully charged destination cells so failure copying the
// denominator cannot publish only the numerator. Source limbs never escape.
func (m *RationalManager) CopyFrom(dst *Rational, source *RationalManager, src *Rational) (_lexErr error) {
	if m != nil && m.lexical.pending() {
		return ErrRationalLexicalPending
	}
	if m == nil || source == nil {
		return arena.ErrConfig
	}
	if m == source {
		return m.Copy(dst, src)
	}
	if dst == src {
		return arena.ErrConfig
	}
	if e := m.check(dst); e != nil {
		return e
	}
	if e := source.check(src); e != nil {
		return e
	}
	work := NewRational()
	defer m.finishLexicalCopy(&work, &_lexErr)
	if e := m.Z.CopyFrom(&work.Num, source.Z, &src.Num); e != nil {
		return e
	}
	if e := m.Z.CopyFrom(&work.Den, source.Z, &src.Den); e != nil {
		return e
	}
	return m.Move(dst, &work)
}
