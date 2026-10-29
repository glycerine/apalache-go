package sat

import (
	utext "github.com/glycerine/g3/pkg/util/text"
)

type satJustification struct {
	val1, val2 uint32
	locator    satClauseOffset
}

func newSATBinaryJustification(l Literal) satJustification {
	return satJustification{val1: uint32(l), val2: 1}
}
func newSATTernaryJustification(a, b Literal) satJustification {
	return satJustification{val1: uint32(a), val2: 2 + (uint32(b) << 3)}
}
func newSATClauseJustification(o satClauseOffset) satJustification {
	return satJustification{val1: o.slot, val2: 3, locator: o}
}
func newSATExternalJustification(i uint32) satJustification {
	return satJustification{val1: i, val2: 4}
}
func (j satJustification) Kind() uint32   { return j.val2 & 7 }
func (j satJustification) None() bool     { return j.val2 == 0 }
func (j satJustification) Binary() bool   { return j.val2 == 1 }
func (j satJustification) Ternary() bool  { return j.Kind() == 2 }
func (j satJustification) Clause() bool   { return j.val2 == 3 }
func (j satJustification) External() bool { return j.val2 == 4 }
func (j satJustification) Literal() (Literal, error) {
	if satStorageSourceDebug && !j.Binary() {
		return 0, ErrSATAssertion
	}
	return Literal(j.val1), nil
}
func (j satJustification) Literal1() (Literal, error) {
	if satStorageSourceDebug && !j.Ternary() {
		return 0, ErrSATAssertion
	}
	return Literal(j.val1), nil
}
func (j satJustification) Literal2() (Literal, error) {
	if satStorageSourceDebug && !j.Ternary() {
		return 0, ErrSATAssertion
	}
	return Literal(j.val2 >> 3), nil
}
func (j satJustification) ClauseOffset() satClauseOffset { o := j.locator; o.slot = j.val1; return o }
func (j satJustification) ExternalIndex() uint32         { return j.val1 }
func (j satJustification) DisplayInto(s *utext.NumericTextStream) error {
	switch j.Kind() {
	case 0:
		return s.AppendCString("none")
	case 1:
		if e := s.AppendCString("binary "); e != nil {
			return e
		}
		l, e := j.Literal()
		if e != nil {
			return e
		}
		return satLiteralInto(s, l)
	case 2:
		if e := s.AppendCString("ternary "); e != nil {
			return e
		}
		l, e := j.Literal1()
		if e != nil {
			return e
		}
		if e = satLiteralInto(s, l); e != nil {
			return e
		}
		if e = s.AppendCString(" "); e != nil {
			return e
		}
		l, e = j.Literal2()
		if e != nil {
			return e
		}
		return satLiteralInto(s, l)
	case 3:
		return s.AppendCString("clause")
	case 4:
		return s.AppendCString("external")
	}
	return nil
}
