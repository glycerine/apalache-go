package lp

// Source: src/util/lp/iterator_on_term_with_basis_var.h, retained mpq instance.

import "github.com/glycerine/g3/pkg/util/numeral"

// LARTermIterator borrows a stable term. A native key cursor records its Dict
// order on Reset; coefficient values remain live borrows. Structural mutation
// requires Reset. Copy failure publishes index but does NOT advance this cursor.
type LARTermIterator struct {
	term          *LARTerm
	basis         uint32
	basisReturned bool
	keys          []uint32
	offset        uint32
	calls         exactVectorCalls
}

func NewLARTermIterator(term *LARTerm, basis uint32) (*LARTermIterator, error) {
	it := &LARTermIterator{term: term, basis: basis}
	if err := it.Reset(); err != nil {
		return nil, err
	}
	return it, nil
}
func (it *LARTermIterator) Size() uint32 { return it.term.Size() + 1 }
func (it *LARTermIterator) Reset() error {
	if it.calls.pending() {
		return ErrExactVectorPending
	}
	if err := it.term.ready(); err != nil {
		return err
	}
	it.keys = it.keys[:0]
	for index := range it.term.coefficients.All() {
		it.keys = append(it.keys, index)
	}
	it.offset = 0
	it.basisReturned = false
	return nil
}
func (it *LARTermIterator) Next(out *numeral.Rational) (index uint32, ok bool, err error) {
	if err = it.term.ready(); err != nil {
		return 0, false, err
	}
	b, err := it.calls.begin(it.term.manager)
	if err != nil {
		return 0, false, err
	}
	defer func() { err = it.calls.after(it.term.manager, b, err) }()
	if !it.basisReturned {
		it.basisReturned = true
		minusOne := numeral.NewRational()
		minusOne.Num = numeral.NewInt(-1)
		if err = it.term.manager.Copy(out, &minusOne); err != nil {
			return 0, false, err
		}
		return it.basis, true, nil
	}
	if uint64(it.offset) >= uint64(len(it.keys)) {
		return 0, false, nil
	}
	index = it.keys[it.offset]
	value, present := it.term.coefficients.Get2(index)
	if !present {
		return index, false, ErrBounds
	}
	if err = it.term.manager.Copy(out, value); err != nil {
		return index, false, err
	}
	it.offset++
	return index, true, nil
}
func (it *LARTermIterator) NextIndex() (uint32, bool, error) {
	if it.calls.pending() {
		return 0, false, ErrExactVectorPending
	}
	if err := it.term.ready(); err != nil {
		return 0, false, err
	}
	if !it.basisReturned {
		it.basisReturned = true
		return it.basis, true, nil
	}
	if uint64(it.offset) >= uint64(len(it.keys)) {
		return 0, false, nil
	}
	index := it.keys[it.offset]
	it.offset++
	return index, true, nil
}
func (it *LARTermIterator) Close() error {
	if err := it.calls.cleanupRaw(it.term.manager); err != nil {
		return err
	}
	return it.calls.cleanupRoots(it.term.manager)
}

var _ RationalLinearIterator = (*LARTermIterator)(nil)

func (it *LARTermIterator) NumeralManager() *numeral.RationalManager { return it.term.manager }
