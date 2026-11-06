package polynomial

// Source: algebraic_numbers.cpp:235-240,337-341,440-444; mpq.h:666-669.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// A borrowed basic_value reference owns neither its header nor its numerals.
type algebraicBasicValueView struct {
	source *AlgebraicRootCore
	cell   *algebraicLocation
	zero   *unumeral.Rational
}

func (v algebraicBasicValueView) read(offset uint64) (unumeral.Int, error) {
	var w [3]uint32
	for i := range w {
		n, e := v.source.config.Allocator.ReadUint32(v.cell.raw, offset+uint64(i)*4)
		if e != nil {
			return unumeral.Int{}, e
		}
		w[i] = n
	}
	return algebraicWordsInt(w[:]), nil
}
func (v algebraicBasicValueView) copyTo(m *AlgebraicRootCore, out *unumeral.Rational, receipt *unumeral.IntegerCopyAllocationReceipt) error {
	q := m.config.Rationals
	if v.cell == nil {
		return q.CopyWithAllocationReceipt(out, v.zero, receipt)
	}
	if e := q.CheckOwnerReady(); e != nil {
		return e
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	n, e := v.read(0)
	if e != nil {
		return e
	}
	z, source := q.Integers(), v.source.config.Rationals.Integers()
	if e = z.CopyFromOwnerWithReceipt(out.BorrowNumerator(), source, &n, receipt); e != nil {
		return e
	}
	d, e := v.read(12)
	if e != nil {
		return e
	}
	return z.CopyFromOwnerWithReceipt(out.BorrowDenominator(), source, &d, receipt)
}
func (m *AlgebraicRootCore) setRationalFromCell(a *Algebraic, v algebraicBasicValueView) (err error) {
	if _, err = m.location(a); err != nil {
		return err
	}
	if err = m.beginConstQ(); err != nil {
		return err
	}
	defer func() { m.constQ.running = false; err = m.finishConstQ(err) }()
	s := &m.constQ
	s.executing = true
	err = v.copyTo(m, &m.localQ, &s.copy)
	s.executing = false
	m.rationalCopy = s.copy
	if err != nil {
		return err
	}
	return m.SetTake(a, &m.localQ)
}
