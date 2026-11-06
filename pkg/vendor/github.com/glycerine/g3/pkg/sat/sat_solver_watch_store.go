package sat

// Native watch slices for retained SAT consumers.

import (
	"github.com/glycerine/g3/pkg/arena"

	utext "github.com/glycerine/g3/pkg/util/text"
	"reflect"
)

// List references retain owner/generation checks across outer-list relocation.
type satSolverWatchListRef struct {
	owner            uint64
	slot, generation uint32
}

// The backing identity is borrowed by iterators. Finalization and real Go slice
// growth invalidate old borrows; native payloads carry no encoded slot metadata.
type satWatchBacking struct {
	data  []satWatched
	alive bool
}
type satSolverWatchListRecord struct {
	backing          *satWatchBacking
	generation, next uint32
	live, retired    bool
}

func (x *satSolverWatchListRecord) values() []satWatched {
	if x.backing == nil {
		return nil
	}
	return x.backing.data
}

type satSolverWatchStore struct {
	self               *satSolverWatchStore
	budget             *arena.Budget
	records            []satSolverWatchListRecord
	sorter             satWatchSortSlice
	origin, ownerBytes uint64
	free               uint32
	closed             bool
}

func newSATSolverWatchStore(b *arena.Budget) (*satSolverWatchStore, error) {
	bytes := uint64(reflect.TypeOf(satSolverWatchStore{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	id, e := satNextOwner()
	if e != nil {
		b.Release(bytes)
		return nil, e
	}
	s := &satSolverWatchStore{budget: b, origin: id, ownerBytes: bytes}
	s.self = s
	return s, nil
}
func (s *satSolverWatchStore) open() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *satSolverWatchStore) record(r satSolverWatchListRef) (*satSolverWatchListRecord, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	if r.owner != s.origin {
		return nil, ErrSATOwner
	}
	if uint64(r.slot) >= uint64(len(s.records)) {
		return nil, arena.ErrInvalidSlot
	}
	x := &s.records[r.slot]
	if !x.live || x.generation != r.generation {
		return nil, arena.ErrInvalidSlot
	}
	return x, nil
}
func (s *satSolverWatchStore) IsLive(r satSolverWatchListRef) bool {
	_, e := s.record(r)
	return e == nil
}
func (s *satSolverWatchStore) NewList() (satSolverWatchListRef, error) {
	if e := s.open(); e != nil {
		return satSolverWatchListRef{}, e
	}
	if s.free != 0 {
		i := s.free - 1
		x := &s.records[i]
		s.free = x.next
		x.next = 0
		x.live = true
		return satSolverWatchListRef{s.origin, i, x.generation}, nil
	}
	if uint64(len(s.records)) >= uint64(^uint32(0)) {
		return satSolverWatchListRef{}, arena.ErrSlotLimit
	}
	i := uint32(len(s.records))
	s.records = append(s.records, satSolverWatchListRecord{generation: 1, live: true})
	return satSolverWatchListRef{s.origin, i, 1}, nil
}
func (s *satSolverWatchStore) Size(r satSolverWatchListRef) (uint32, error) {
	x, e := s.record(r)
	if e != nil {
		return 0, e
	}
	return uint32(len(x.values())), nil
}
func satWatchLogicalBounds(i uint32, n int) error {
	if uint64(i) < uint64(n) {
		return nil
	}
	if satStorageSourceDebug {
		return ErrSATAssertion
	}
	return arena.ErrBounds
}
func (s *satSolverWatchStore) Read(r satSolverWatchListRef, i uint32) (satWatched, error) {
	x, e := s.record(r)
	if e != nil {
		return satWatched{}, e
	}
	if e = satWatchLogicalBounds(i, len(x.values())); e != nil {
		return satWatched{}, e
	}
	return x.backing.data[i], nil
}
func (s *satSolverWatchStore) Write(r satSolverWatchListRef, i uint32, w satWatched) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	if e = satWatchLogicalBounds(i, len(x.values())); e != nil {
		return e
	}
	x.backing.data[i] = w
	return nil
}
func (s *satSolverWatchStore) Push(r satSolverWatchListRef, w satWatched) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	old := x.values()
	if uint64(len(old)) >= uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	next := append(old, w)
	if x.backing == nil || cap(old) != cap(next) {
		if x.backing != nil {
			x.backing.alive = false
			x.backing.data = nil
		}
		x.backing = &satWatchBacking{data: next, alive: true}
	} else {
		x.backing.data = next
	}
	return nil
}
func (s *satSolverWatchStore) SetEnd(r satSolverWatchListRef, n uint32) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	data := x.values()
	if uint64(n) > uint64(len(data)) {
		return arena.ErrBounds
	}
	clear(data[n:])
	if x.backing != nil {
		x.backing.data = data[:n]
	}
	return nil
}
func (s *satSolverWatchStore) Shrink(r satSolverWatchListRef, n uint32) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	if satStorageSourceDebug && uint64(n) > uint64(len(x.values())) {
		return ErrSATAssertion
	}
	return s.SetEnd(r, n)
}
func (s *satSolverWatchStore) eraseAt(r satSolverWatchListRef, i uint32) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	data := x.values()
	if e = satWatchLogicalBounds(i, len(data)); e != nil {
		return e
	}
	copy(data[i:], data[i+1:])
	clear(data[len(data)-1:])
	x.backing.data = data[:len(data)-1]
	return nil
}
func (s *satSolverWatchStore) Erase(r satSolverWatchListRef, w satWatched) (bool, error) {
	x, e := s.record(r)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < uint32(len(x.values())); i++ {
		v, e := s.Read(r, i)
		if e != nil {
			return false, e
		}
		if v.Equal(w) {
			return true, s.eraseAt(r, i)
		}
	}
	return false, nil
}
func (s *satSolverWatchStore) EraseClauseWatch(r satSolverWatchListRef, o satClauseOffset) (bool, error) {
	x, e := s.record(r)
	if e != nil {
		return false, e
	}
	for i, w := range x.values() {
		if w.Clause() {
			q, e := w.ClauseOffset()
			if e != nil {
				return false, e
			}
			if q == o {
				return true, s.eraseAt(r, uint32(i))
			}
		}
	}
	return false, nil
}
func (s *satSolverWatchStore) Finalize(r satSolverWatchListRef) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	if x.backing != nil {
		x.backing.alive = false
		x.backing.data = nil
		x.backing = nil
	}
	return nil
}
func (s *satSolverWatchStore) Free(r satSolverWatchListRef) error {
	if e := s.Finalize(r); e != nil {
		return e
	}
	x, e := s.record(r)
	if e != nil {
		return e
	}
	x.live = false
	if x.generation == ^uint32(0) {
		x.retired = true
		return nil
	}
	x.generation++
	x.next = s.free
	s.free = r.slot + 1
	return nil
}
func (s *satSolverWatchStore) Close() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return nil
	}
	for i := range s.records {
		if b := s.records[i].backing; b != nil {
			b.alive = false
			b.data = nil
		}
	}
	s.records = nil
	s.budget.Release(s.ownerBytes)
	s.ownerBytes = 0
	s.closed = true
	return nil
}

type satSolverWatchIterator struct {
	store      *satSolverWatchStore
	backing    *satWatchBacking
	index, end uint32
}

func (s *satSolverWatchStore) Iter(r satSolverWatchListRef) (satSolverWatchIterator, error) {
	x, e := s.record(r)
	if e != nil {
		return satSolverWatchIterator{}, e
	}
	return satSolverWatchIterator{store: s, backing: x.backing, end: uint32(len(x.values()))}, nil
}
func (i *satSolverWatchIterator) Next() (satWatched, bool, error) {
	if i == nil || i.store == nil {
		return satWatched{}, false, ErrSATOwner
	}
	if e := i.store.open(); e != nil {
		return satWatched{}, false, e
	}
	if i.index == i.end {
		return satWatched{}, false, nil
	}
	w, e := satPropagationCursorRead(i, i.index)
	if e != nil {
		return w, false, e
	}
	i.index++
	return w, true, nil
}
func (s *satSolverWatchStore) Contains(r satSolverWatchListRef, w satWatched) (bool, error) {
	x, e := s.record(r)
	if e != nil {
		return false, e
	}
	for _, v := range x.values() {
		if v.Equal(w) {
			return true, nil
		}
	}
	return false, nil
}

type satSolverWatchStoreStats struct {
	Lists, Retired           uint32
	OwnerBytes, ChargedBytes uint64
}

func (s *satSolverWatchStore) Stats() satSolverWatchStoreStats {
	if s == nil || s.self != s {
		return satSolverWatchStoreStats{}
	}
	out := satSolverWatchStoreStats{OwnerBytes: s.ownerBytes, ChargedBytes: s.ownerBytes}
	for _, x := range s.records {
		if x.live {
			out.Lists++
		}
		if x.retired {
			out.Retired++
		}
	}
	return out
}
func (v *satSolverWatchStore) DisplayInto(r satSolverWatchListRef, a *ClauseAllocator, s *utext.NumericTextStream) error {
	x, e := v.record(r)
	if e != nil {
		return e
	}
	for i := uint32(0); i < uint32(len(x.values())); i++ {
		w, e := v.Read(r, i)
		if e != nil {
			return e
		}
		if i != 0 {
			if e := s.AppendCString(" "); e != nil {
				return e
			}
		}
		switch w.Kind() {
		case 0:
			l, e := w.Literal()
			if e != nil {
				return e
			}
			if e = satLiteralInto(s, l); e != nil {
				return e
			}
			if w.LearnedUnchecked() {
				if e = s.AppendCString("*"); e != nil {
					return e
				}
			}
		case 1:
			if e := s.AppendCString("("); e != nil {
				return e
			}
			l, e := w.Literal1()
			if e != nil {
				return e
			}
			if e = satLiteralInto(s, l); e != nil {
				return e
			}
			if e = s.AppendCString(" "); e != nil {
				return e
			}
			l, e = w.Literal2()
			if e != nil {
				return e
			}
			if e = satLiteralInto(s, l); e != nil {
				return e
			}
			if e = s.AppendCString(")"); e != nil {
				return e
			}
		case 2:
			if e := s.AppendCString("("); e != nil {
				return e
			}
			l, e := w.BlockedLiteral()
			if e != nil {
				return e
			}
			if e = satLiteralInto(s, l); e != nil {
				return e
			}
			if e = s.AppendCString(" "); e != nil {
				return e
			}
			o, e := w.ClauseOffset()
			if e != nil {
				return e
			}
			r, e := a.GetClause(o)
			if e != nil {
				return e
			}
			if e = a.DisplayInto(r, s); e != nil {
				return e
			}
			if e = s.AppendCString(")"); e != nil {
				return e
			}
		case 3:
			n, e := w.ExternalIndex()
			if e != nil {
				return e
			}
			if e = s.AppendUint32(n); e != nil {
				return e
			}
		}
	}
	return nil
}

func (s *satSolverWatchStore) EraseTernaryWatch(r satSolverWatchListRef, a, b Literal) error {
	w, e := newSATTernaryWatch(a, b)
	if e != nil {
		return e
	}
	_, e = s.Erase(r, w)
	return e
}
