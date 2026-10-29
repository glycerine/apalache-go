package sat

// Translation of selected LAZY_USE_LIST in src/sat/sat_clause_use_list.h/.cpp.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// The cold list owns its actual vector, never the clauses referenced by it.
// Its clause allocator(s) are borrowed through the vector and must outlive all
// was_removed callbacks. liveSize differs from physical vector length during
// lazy deletion. It intentionally preserves source unsigned underflow.
type satClauseUseList struct {
	self       *satClauseUseList
	vector     *satClauseVector
	budget     *arena.Budget
	liveSize   uint32
	ownerBytes uint64
	closed     bool
}

func newSATClauseUseList(a *ClauseAllocator, b *arena.Budget) (*satClauseUseList, error) {
	if e := a.open(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(satClauseUseList{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	v, e := NewSATClauseVector(a, b)
	if e != nil {
		b.Release(n)
		return nil, e
	}
	u := &satClauseUseList{vector: v, budget: b, ownerBytes: n}
	u.self = u
	return u, nil
}
func (u *satClauseUseList) open() error {
	if u == nil || u.self != u {
		return ErrSATOwner
	}
	if u.closed {
		return arena.ErrClosed
	}
	return nil
}
func (u *satClauseUseList) Size() uint32 {
	if u == nil || u.closed {
		return 0
	}
	return u.liveSize
}
func (u *satClauseUseList) Empty() bool { return u.Size() == 0 }
func satUseListRemoved(v *satClauseVector, r StoredClauseRef) (bool, error) {
	a, e := v.resolve(r.owner)
	if e != nil {
		return false, e
	}
	return a.Removed(r)
}
func (u *satClauseUseList) Insert(r StoredClauseRef) error {
	if e := u.open(); e != nil {
		return e
	}
	if satStorageSourceDebug {
		found, e := u.vector.Contains(r)
		if e != nil {
			return e
		}
		if found {
			return ErrSATAssertion
		}
		removed, e := satUseListRemoved(u.vector, r)
		if e != nil {
			return e
		}
		if removed {
			return ErrSATAssertion
		}
	}
	if e := u.vector.Push(r); e != nil {
		return e
	}
	u.liveSize++
	return nil
}
func (u *satClauseUseList) EraseNotRemoved(r StoredClauseRef) error {
	if e := u.open(); e != nil {
		return e
	}
	if satStorageSourceDebug {
		found, e := u.vector.Contains(r)
		if e != nil {
			return e
		}
		if !found {
			return ErrSATAssertion
		}
		removed, e := satUseListRemoved(u.vector, r)
		if e != nil {
			return e
		}
		if removed {
			return ErrSATAssertion
		}
	}
	if _, e := u.vector.Erase(r); e != nil {
		return e
	}
	u.liveSize--
	return nil
}
func (u *satClauseUseList) Erase(r StoredClauseRef) error {
	if e := u.open(); e != nil {
		return e
	}
	if satStorageSourceDebug {
		found, e := u.vector.Contains(r)
		if e != nil {
			return e
		}
		if !found {
			return ErrSATAssertion
		}
		removed, e := satUseListRemoved(u.vector, r)
		if e != nil {
			return e
		}
		if !removed {
			return ErrSATAssertion
		}
	}
	u.liveSize--
	return nil
}
func (u *satClauseUseList) Reset() error {
	if e := u.open(); e != nil {
		return e
	}
	if e := u.vector.Finalize(); e != nil {
		return e
	}
	u.liveSize = 0
	return nil
}

// The counting loop is real source code in release too; only its final SASSERT
// is debug-only. A stale clause therefore fails at the actual payload read in
// both profiles rather than being hidden by an unconditional true result.
func (u *satClauseUseList) CheckInvariant() (bool, error) {
	if e := u.open(); e != nil {
		return false, e
	}
	var count uint32
	for i := uint32(0); i < u.vector.Size(); i++ {
		r, e := u.vector.Get(i)
		if e != nil {
			return false, e
		}
		removed, e := satUseListRemoved(u.vector, r)
		if e != nil {
			return false, e
		}
		if !removed {
			count++
		}
	}
	if satStorageSourceDebug && count != u.liveSize {
		return false, ErrSATAssertion
	}
	return true, nil
}

// Implicit source member copy: vector copy precedes the scalar liveSize copy.
// The source vector assignment's finalize-before-allocation failure prefix is
// visible; liveSize is not assigned if that member assignment fails.
func (u *satClauseUseList) Clone(b *arena.Budget) (*satClauseUseList, error) {
	if e := u.open(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(satClauseUseList{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	v, e := u.vector.Clone(b)
	if e != nil {
		b.Release(n)
		return nil, e
	}
	r := &satClauseUseList{vector: v, budget: b, liveSize: u.liveSize, ownerBytes: n}
	r.self = r
	return r, nil
}
func (u *satClauseUseList) Assign(src *satClauseUseList) error {
	if e := u.open(); e != nil {
		return e
	}
	if e := src.open(); e != nil {
		return e
	}
	if e := u.vector.Assign(src.vector); e != nil {
		return e
	}
	u.liveSize = src.liveSize
	return nil
}
func (u *satClauseUseList) Close() error {
	if u == nil || u.self != u {
		return ErrSATOwner
	}
	if u.closed {
		return nil
	}
	if e := u.vector.Close(); e != nil {
		return e
	}
	u.budget.Release(u.ownerBytes)
	u.ownerBytes = 0
	u.closed = true
	return nil
}

// A by-value lexical iterator: no heap owner/header is allocated. It borrows
// the stable vector object, NOT its original backing. Source copy elision and
// Go explicit Close require that callers do not copy a live owning iterator.
// The clause-use list itself need not be modified by const mk_iterator.
type satClauseUseListIterator struct {
	vector     *satClauseVector
	size, i, j uint32
	closed     bool
}

func (u *satClauseUseList) Begin() (satClauseUseListIterator, error) {
	if e := u.open(); e != nil {
		return satClauseUseListIterator{}, e
	}
	if e := u.vector.open(); e != nil {
		return satClauseUseListIterator{}, e
	}
	it := satClauseUseListIterator{vector: u.vector, size: u.vector.Size()}
	e := it.consume()
	return it, e
}

// Plain source m_i==m_size: no vector/lifetime dereference or epoch check.
func (it *satClauseUseListIterator) AtEnd() bool { return it.i == it.size }
func (it *satClauseUseListIterator) consume() error {
	for {
		if it.i == it.size {
			return nil
		}
		r, e := it.vector.Get(it.i)
		if e != nil {
			return e
		}
		removed, e := satUseListRemoved(it.vector, r)
		if e != nil {
			return e
		}
		if !removed {
			return it.vector.Set(it.j, r)
		}
		it.i++
	}
}

// Current copies only the retained source clause reference. No extra removed
// callback is invented; an actual later payload access validates its lifetime.
func (it *satClauseUseListIterator) Current() (StoredClauseRef, error) {
	if it.closed {
		return StoredClauseRef{}, arena.ErrClosed
	}
	if satStorageSourceDebug && it.AtEnd() {
		return StoredClauseRef{}, ErrSATAssertion
	}
	if it.vector == nil {
		return StoredClauseRef{}, ErrSATOwner
	}
	return it.vector.Get(it.i)
}
func (it *satClauseUseListIterator) Next() error {
	if it.closed {
		return arena.ErrClosed
	}
	if satStorageSourceDebug {
		if it.AtEnd() {
			return ErrSATAssertion
		}
		if it.vector == nil {
			return ErrSATOwner
		}
		r, e := it.vector.Get(it.i)
		if e != nil {
			return e
		}
		removed, e := satUseListRemoved(it.vector, r)
		if e != nil {
			return e
		}
		if removed {
			return ErrSATAssertion
		}
	}
	// Source release next has no dereference until consume; preserve its plain
	// unsigned increments before any actual read/write guard in that method.
	it.i++
	it.j++
	return it.consume()
}
func (it *satClauseUseListIterator) Close() error {
	if it.closed {
		return nil
	}
	for it.i < it.size {
		if e := it.Next(); e != nil {
			return e
		}
	}
	if it.vector == nil {
		return ErrSATOwner
	}
	if e := it.vector.Shrink(it.j); e != nil {
		return e
	}
	it.closed = true
	return nil
}
