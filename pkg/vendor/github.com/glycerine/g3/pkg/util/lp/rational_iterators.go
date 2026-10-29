package lp

// Source: src/util/lp/linear_combination_iterator.h and retained exact iterators.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalLinearIterator is the retained precise project iterator interface.
// Next copies into initialized caller-owned out. An error may have advanced the
// cursor and published the returned index; ok is true only for a completed term.
// Borrowed containers and their numeral manager must outlive the iterator and
// remain stable during traversal. Close owns only the iterator's numeric calls.
// Source clone and the vector-of-pairs iterator have no retained call sites.
type RationalLinearIterator interface {
	// NumeralManager identifies the borrowed numeric owner before any value copy.
	NumeralManager() *numeral.RationalManager
	Next(out *numeral.Rational) (index uint32, ok bool, err error)
	NextIndex() (index uint32, ok bool, err error)
	Reset() error
	Size() uint32
	Close() error
}

type RationalRowIterator struct {
	row     *[]RationalRowCell
	offset  uint32
	manager *numeral.RationalManager
	calls   exactVectorCalls
}

func NewRationalRowIterator(m *RationalStaticMatrix, row uint32) (*RationalRowIterator, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	if uint64(row) >= uint64(len(m.Rows)) {
		return nil, ErrBounds
	}
	return &RationalRowIterator{row: &m.Rows[row], manager: m.manager}, nil
}
func (it *RationalRowIterator) Size() uint32 { return uint32(len(*it.row)) }
func (it *RationalRowIterator) Reset() error {
	if it.calls.pending() {
		return ErrExactVectorPending
	}
	it.offset = 0
	return nil
}
func (it *RationalRowIterator) Next(out *numeral.Rational) (index uint32, ok bool, err error) {
	b, err := it.calls.begin(it.manager)
	if err != nil {
		return 0, false, err
	}
	defer func() { err = it.calls.after(it.manager, b, err) }()
	if it.offset == uint32(len(*it.row)) {
		return 0, false, nil
	}
	if uint64(it.offset) >= uint64(len(*it.row)) {
		return 0, false, ErrBounds
	}
	c := &(*it.row)[it.offset]
	it.offset++
	index = c.Column // Source publishes index before the owning value copy.
	err = it.manager.Copy(out, &c.Value)
	return index, err == nil, err
}
func (it *RationalRowIterator) NextIndex() (uint32, bool, error) {
	if it.calls.pending() {
		return 0, false, ErrExactVectorPending
	}
	if it.offset == uint32(len(*it.row)) {
		return 0, false, nil
	}
	if uint64(it.offset) >= uint64(len(*it.row)) {
		return 0, false, ErrBounds
	}
	index := (*it.row)[it.offset].Column
	it.offset++
	return index, true, nil
}
func (it *RationalRowIterator) Close() error {
	if err := it.calls.cleanupRaw(it.manager); err != nil {
		return err
	}
	return it.calls.cleanupRoots(it.manager)
}

type RationalColumnIterator struct {
	column *[]ColumnCell
	matrix *RationalStaticMatrix
	offset int32
	calls  exactVectorCalls
}

func NewRationalColumnIterator(m *RationalStaticMatrix, column uint32) (*RationalColumnIterator, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	if uint64(column) >= uint64(len(m.Columns)) {
		return nil, ErrBounds
	}
	return &RationalColumnIterator{column: &m.Columns[column], matrix: m, offset: -1}, nil
}
func (it *RationalColumnIterator) Size() uint32 { return uint32(len(*it.column)) }
func (it *RationalColumnIterator) Reset() error {
	if it.calls.pending() {
		return ErrExactVectorPending
	}
	it.offset = -1
	return nil
}
func (it *RationalColumnIterator) Next(out *numeral.Rational) (index uint32, ok bool, err error) {
	b, err := it.calls.begin(it.matrix.manager)
	if err != nil {
		return 0, false, err
	}
	defer func() { err = it.calls.after(it.matrix.manager, b, err) }()
	it.offset++ // Source increments even at each repeated exhaustion.
	if it.offset < 0 {
		return 0, false, ErrBounds
	}
	if uint64(it.offset) >= uint64(len(*it.column)) {
		return 0, false, nil
	}
	c := (*it.column)[it.offset]
	value, err := it.matrix.ColumnValue(c)
	if err != nil {
		return 0, false, err
	}
	if err = it.matrix.manager.Copy(out, value); err != nil {
		return 0, false, err
	}
	return c.Row, true, nil // Source publishes row only after its value copy.
}
func (it *RationalColumnIterator) NextIndex() (uint32, bool, error) {
	if it.calls.pending() {
		return 0, false, ErrExactVectorPending
	}
	it.offset++
	if it.offset < 0 {
		return 0, false, ErrBounds
	}
	if uint64(it.offset) >= uint64(len(*it.column)) {
		return 0, false, nil
	}
	return (*it.column)[it.offset].Row, true, nil
}
func (it *RationalColumnIterator) Close() error {
	if err := it.calls.cleanupRaw(it.matrix.manager); err != nil {
		return err
	}
	return it.calls.cleanupRoots(it.matrix.manager)
}

type RationalIndexedIterator struct {
	vector *IndexedRationalVector
	offset uint32
	calls  exactVectorCalls
}

func NewRationalIndexedIterator(v *IndexedRationalVector) *RationalIndexedIterator {
	return &RationalIndexedIterator{vector: v}
}
func (it *RationalIndexedIterator) Size() uint32 { return uint32(len(it.vector.Index)) }
func (it *RationalIndexedIterator) Reset() error {
	if it.calls.pending() {
		return ErrExactVectorPending
	}
	it.offset = 0
	return nil
}
func (it *RationalIndexedIterator) Next(out *numeral.Rational) (index uint32, ok bool, err error) {
	b, err := it.calls.begin(it.vector.manager)
	if err != nil {
		return 0, false, err
	}
	defer func() { err = it.calls.after(it.vector.manager, b, err) }()
	if uint64(it.offset) >= uint64(len(it.vector.Index)) {
		return 0, false, nil
	}
	index = it.vector.Index[it.offset]
	it.offset++
	if uint64(index) >= uint64(len(it.vector.Data)) {
		return index, false, ErrBounds
	}
	err = it.vector.manager.Copy(out, &it.vector.Data[index])
	return index, err == nil, err
}
func (it *RationalIndexedIterator) NextIndex() (uint32, bool, error) {
	if it.calls.pending() {
		return 0, false, ErrExactVectorPending
	}
	if uint64(it.offset) >= uint64(len(it.vector.Index)) {
		return 0, false, nil
	}
	index := it.vector.Index[it.offset]
	it.offset++
	return index, true, nil
}
func (it *RationalIndexedIterator) Close() error {
	if err := it.calls.cleanupRaw(it.vector.manager); err != nil {
		return err
	}
	return it.calls.cleanupRoots(it.vector.manager)
}

type RationalPivotRowIterator struct {
	inner         *RationalIndexedIterator
	basis         uint32
	basisReturned bool
	calls         exactVectorCalls
}

func NewRationalPivotRowIterator(v *IndexedRationalVector, basis uint32) *RationalPivotRowIterator {
	return &RationalPivotRowIterator{inner: NewRationalIndexedIterator(v), basis: basis}
}

// Source size excludes the separately returned unit basis coefficient.
func (it *RationalPivotRowIterator) Size() uint32 { return it.inner.Size() }
func (it *RationalPivotRowIterator) Reset() error {
	if it.calls.pending() || it.inner.calls.pending() {
		return ErrExactVectorPending
	}
	it.basisReturned = false
	return it.inner.Reset()
}
func (it *RationalPivotRowIterator) Next(out *numeral.Rational) (index uint32, ok bool, err error) {
	if it.calls.pending() || it.inner.calls.pending() {
		return 0, false, ErrExactVectorPending
	}
	if it.basisReturned {
		return it.inner.Next(out)
	}
	b, err := it.calls.begin(it.inner.vector.manager)
	if err != nil {
		return 0, false, err
	}
	defer func() { err = it.calls.after(it.inner.vector.manager, b, err) }()
	it.basisReturned = true
	one := numeral.NewRational()
	one.Num = numeral.NewInt(1)
	if err = it.inner.vector.manager.Copy(out, &one); err != nil {
		return 0, false, err
	}
	return it.basis, true, nil
}
func (it *RationalPivotRowIterator) NextIndex() (uint32, bool, error) {
	if it.calls.pending() || it.inner.calls.pending() {
		return 0, false, ErrExactVectorPending
	}
	if it.basisReturned {
		return it.inner.NextIndex()
	}
	it.basisReturned = true
	return it.basis, true, nil
}
func (it *RationalPivotRowIterator) Close() error {
	if err := it.calls.cleanupRaw(it.inner.vector.manager); err != nil {
		return err
	}
	if err := it.calls.cleanupRoots(it.inner.vector.manager); err != nil {
		return err
	}
	return it.inner.Close()
}

var _ RationalLinearIterator = (*RationalRowIterator)(nil)
var _ RationalLinearIterator = (*RationalColumnIterator)(nil)
var _ RationalLinearIterator = (*RationalIndexedIterator)(nil)
var _ RationalLinearIterator = (*RationalPivotRowIterator)(nil)

func (it *RationalRowIterator) NumeralManager() *numeral.RationalManager    { return it.manager }
func (it *RationalColumnIterator) NumeralManager() *numeral.RationalManager { return it.matrix.manager }
func (it *RationalIndexedIterator) NumeralManager() *numeral.RationalManager {
	return it.vector.manager
}
func (it *RationalPivotRowIterator) NumeralManager() *numeral.RationalManager {
	return it.inner.vector.manager
}
