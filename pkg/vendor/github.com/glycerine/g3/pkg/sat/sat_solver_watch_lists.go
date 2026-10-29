package sat

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Native outer slices retain checked child references. Relocation copies tokens
// without duplicating ownership; native backing has no simulated budget charge.
type satSolverWatchLists struct {
	self                  *satSolverWatchLists
	store                 *satSolverWatchStore
	budget                *arena.Budget
	lists                 []satSolverWatchListRef
	ownerBytes, backingID uint64
	closed                bool
}
type satSolverWatchListsStats struct {
	OwnerBytes, StoreBytes, TotalBytes uint64
	Size, Capacity                     uint32
}

func newSATSolverWatchLists(b *arena.Budget) (*satSolverWatchLists, error) {
	n := uint64(reflect.TypeOf(satSolverWatchLists{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	store, e := newSATSolverWatchStore(b)
	if e != nil {
		b.Release(n)
		return nil, e
	}
	w := &satSolverWatchLists{store: store, budget: b, ownerBytes: n, backingID: 1}
	w.self = w
	return w, nil
}
func (w *satSolverWatchLists) open() error {
	if w == nil || w.self != w {
		return ErrSATOwner
	}
	if w.closed {
		return arena.ErrClosed
	}
	return nil
}
func (w *satSolverWatchLists) PushEmpty() error {
	if e := w.open(); e != nil {
		return e
	}
	if len(w.lists) == cap(w.lists) && w.backingID == ^uint64(0) {
		return arena.ErrSlotLimit
	}
	oldCapacity := cap(w.lists)
	r, e := w.store.NewList()
	if e != nil {
		return e
	}
	w.lists = append(w.lists, r)
	if cap(w.lists) != oldCapacity {
		w.backingID++
	}
	return nil
}
func (w *satSolverWatchLists) Stats() satSolverWatchListsStats {
	if w == nil || w.closed {
		return satSolverWatchListsStats{}
	}
	store := w.store.Stats().ChargedBytes
	return satSolverWatchListsStats{w.ownerBytes, store, w.ownerBytes + store, uint32(len(w.lists)), uint32(cap(w.lists))}
}
func (w *satSolverWatchLists) Get(i uint32) (satSolverWatchListView, error) {
	if e := w.open(); e != nil {
		return satSolverWatchListView{}, e
	}
	if satStorageSourceDebug && uint64(i) >= uint64(len(w.lists)) {
		return satSolverWatchListView{}, ErrSATAssertion
	}
	if uint64(i) >= uint64(len(w.lists)) {
		return satSolverWatchListView{}, arena.ErrBounds
	}
	return satSolverWatchListView{w, i, w.backingID}, nil
}

type satSolverWatchListView struct {
	owner     *satSolverWatchLists
	index     uint32
	backingID uint64
}

func (v satSolverWatchListView) ref() (satSolverWatchListRef, error) {
	if e := v.owner.open(); e != nil {
		return satSolverWatchListRef{}, e
	}
	if v.backingID != v.owner.backingID {
		return satSolverWatchListRef{}, ErrSATOwner
	}
	if uint64(v.index) >= uint64(len(v.owner.lists)) {
		return satSolverWatchListRef{}, arena.ErrBounds
	}
	r := v.owner.lists[v.index]
	if !v.owner.store.IsLive(r) {
		return satSolverWatchListRef{}, arena.ErrInvalidSlot
	}
	return r, nil
}
func (v satSolverWatchListView) Size() (uint32, error) {
	r, e := v.ref()
	if e != nil {
		return 0, e
	}
	return v.owner.store.Size(r)
}
func (v satSolverWatchListView) Push(w satWatched) error {
	r, e := v.ref()
	if e != nil {
		return e
	}
	return v.owner.store.Push(r, w)
}
func (v satSolverWatchListView) Read(i uint32) (satWatched, error) {
	r, e := v.ref()
	if e != nil {
		return satWatched{}, e
	}
	return v.owner.store.Read(r, i)
}
func (v satSolverWatchListView) Write(i uint32, w satWatched) error {
	r, e := v.ref()
	if e != nil {
		return e
	}
	return v.owner.store.Write(r, i, w)
}
func (v satSolverWatchListView) Finalize() error {
	r, e := v.ref()
	if e != nil {
		return e
	}
	return v.owner.store.Finalize(r)
}
func (v satSolverWatchListView) SetEnd(n uint32) error {
	r, e := v.ref()
	if e != nil {
		return e
	}
	return v.owner.store.SetEnd(r, n)
}
func (w *satSolverWatchLists) Finalize() error {
	if e := w.open(); e != nil {
		return e
	}
	if w.backingID == ^uint64(0) {
		return arena.ErrSlotLimit
	}
	for i := range w.lists {
		if w.lists[i] != (satSolverWatchListRef{}) {
			if e := w.store.Free(w.lists[i]); e != nil {
				return e
			}
			w.lists[i] = satSolverWatchListRef{}
		}
	}
	w.lists = nil
	w.backingID++
	return nil
}
func (w *satSolverWatchLists) Close() error {
	if w == nil || w.self != w {
		return ErrSATOwner
	}
	if w.closed {
		return nil
	}
	if e := w.Finalize(); e != nil {
		return e
	}
	if e := w.store.Close(); e != nil {
		return e
	}
	w.budget.Release(w.ownerBytes)
	w.ownerBytes = 0
	w.closed = true
	return nil
}

// Source vector<watch_list>::shrink destroys ascending removed members, then
// publishes size. The physical outer backing and surviving tokens stay put.
func (w *satSolverWatchLists) Shrink(n uint32) error {
	if e := w.open(); e != nil {
		return e
	}
	if satStorageSourceDebug && uint64(n) > uint64(len(w.lists)) {
		return ErrSATAssertion
	}
	if uint64(n) > uint64(len(w.lists)) {
		return arena.ErrBounds
	}
	old := uint32(len(w.lists))
	for i := n; i < old; i++ {
		r := w.lists[i]
		if r != (satSolverWatchListRef{}) {
			if e := w.store.Free(r); e != nil {
				return e
			}
			w.lists[i] = satSolverWatchListRef{}
		}
	}
	w.lists = w.lists[:n]
	return nil
}
