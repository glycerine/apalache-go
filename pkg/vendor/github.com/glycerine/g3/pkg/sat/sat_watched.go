package sat

type satWatched struct {
	val1, val2 uint32
	locator    satClauseOffset
}

func newSATBinaryWatch(l Literal, learned bool) (satWatched, error) {
	n := uint32(0)
	if learned {
		n = 4
	}
	return satWatched{val1: uint32(l), val2: n}, nil
}
func newSATTernaryWatch(a, b Literal) (satWatched, error) {
	if satStorageSourceDebug && a == b {
		return satWatched{}, ErrSATAssertion
	}
	if a > b {
		a, b = b, a
	}
	w := satWatched{val1: uint32(a), val2: 1 + (uint32(b) << 2)}
	if satStorageSourceDebug && Literal(w.val2>>2) != b {
		return w, ErrSATAssertion
	}
	return w, nil
}
func newSATClauseWatch(l Literal, o satClauseOffset) (satWatched, error) {
	w := satWatched{val1: o.slot, val2: 2 + (uint32(l) << 2), locator: o}
	if satStorageSourceDebug && Literal(w.val2>>2) != l {
		return w, ErrSATAssertion
	}
	return w, nil
}
func newSATExternalWatch(i uint32) (satWatched, error) {
	w := satWatched{val1: i, val2: 3}
	if satStorageSourceDebug && i != 3 {
		return w, ErrSATAssertion
	}
	return w, nil
}
func (w satWatched) Kind() uint32   { return w.val2 & 3 }
func (w satWatched) Binary() bool   { return w.Kind() == 0 }
func (w satWatched) Ternary() bool  { return w.Kind() == 1 }
func (w satWatched) Clause() bool   { return w.Kind() == 2 }
func (w satWatched) External() bool { return w.Kind() == 3 }
func (w satWatched) check(k uint32) error {
	if satStorageSourceDebug && w.Kind() != k {
		return ErrSATAssertion
	}
	return nil
}
func (w satWatched) Literal() (Literal, error) { return Literal(w.val1), w.check(0) }
func (w *satWatched) SetLiteral(l Literal) error {
	if e := w.check(0); e != nil {
		return e
	}
	w.val1 = uint32(l)
	return nil
}
func (w satWatched) LearnedUnchecked() bool { return w.val2>>2 == 1 }
func (w satWatched) Learned() (bool, error) { return w.LearnedUnchecked(), w.check(0) }
func (w satWatched) BinaryNonLearned() bool { return w.val2 == 0 }
func (w *satWatched) MarkNotLearned() error {
	if satStorageSourceDebug && (!w.Binary() || !w.LearnedUnchecked()) {
		return ErrSATAssertion
	}
	w.val2 = 0
	return nil
}
func (w satWatched) Literal1() (Literal, error) { return Literal(w.val1), w.check(1) }
func (w satWatched) Literal2() (Literal, error) { return Literal(w.val2 >> 2), w.check(1) }
func (w satWatched) ClauseOffset() (satClauseOffset, error) {
	o := w.locator
	o.slot = w.val1
	return o, w.check(2)
}
func (w satWatched) BlockedLiteral() (Literal, error) { return Literal(w.val2 >> 2), w.check(2) }
func (w *satWatched) SetClauseOffset(o satClauseOffset) error {
	if e := w.check(2); e != nil {
		return e
	}
	w.val1 = o.slot
	w.locator = o
	return nil
}
func (w *satWatched) SetBlockedLiteral(l Literal) error {
	if e := w.check(2); e != nil {
		return e
	}
	w.val2 = 2 + (uint32(l) << 2)
	return nil
}
func (w *satWatched) SetClause(l Literal, o satClauseOffset) {
	w.val1 = o.slot
	w.val2 = 2 + (uint32(l) << 2)
	w.locator = o
}
func (w satWatched) ExternalIndex() (uint32, error) { return w.val2, w.check(3) }
func (w satWatched) Equal(v satWatched) bool {
	if w.val1 != v.val1 || w.val2 != v.val2 {
		return false
	}
	return !w.Clause() || w.locator == v.locator
}
func satWatchedLess(a, b satWatched) bool {
	if b.Binary() {
		return false
	}
	if a.Binary() {
		return true
	}
	if b.Ternary() {
		return false
	}
	return a.Ternary()
}
