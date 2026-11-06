package sat

// Native literal slices for retained solver model/simplifier consumers.

import (
	"github.com/glycerine/g3/pkg/arena"
	"reflect"
)

type satSolverLiteralListRef struct {
	owner            uint64
	slot, generation uint32
}

// Captured spans borrow this identity; growth/free retire it and release data.
type satLiteralBacking struct {
	data  []Literal
	alive bool
}
type satSolverLiteralListRecord struct {
	backing          *satLiteralBacking
	generation, next uint32
	live, retired    bool
}

func (x *satSolverLiteralListRecord) values() []Literal {
	if x.backing == nil {
		return nil
	}
	return x.backing.data
}

type satSolverLiteralStore struct {
	self               *satSolverLiteralStore
	budget             *arena.Budget
	records            []satSolverLiteralListRecord
	origin, ownerBytes uint64
	free               uint32
	closed             bool
}

func newSATSolverLiteralStore(b *arena.Budget) (*satSolverLiteralStore, error) {
	bytes := uint64(reflect.TypeOf(satSolverLiteralStore{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	id, e := satNextOwner()
	if e != nil {
		b.Release(bytes)
		return nil, e
	}
	s := &satSolverLiteralStore{budget: b, origin: id, ownerBytes: bytes}
	s.self = s
	return s, nil
}
func (s *satSolverLiteralStore) open() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *satSolverLiteralStore) record(r satSolverLiteralListRef) (*satSolverLiteralListRecord, error) {
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
func (s *satSolverLiteralStore) IsLive(r satSolverLiteralListRef) bool {
	_, e := s.record(r)
	return e == nil
}
func (s *satSolverLiteralStore) NewList() (satSolverLiteralListRef, error) {
	if e := s.open(); e != nil {
		return satSolverLiteralListRef{}, e
	}
	if s.free != 0 {
		i := s.free - 1
		x := &s.records[i]
		s.free = x.next
		x.next = 0
		x.live = true
		return satSolverLiteralListRef{s.origin, i, x.generation}, nil
	}
	if uint64(len(s.records)) >= uint64(^uint32(0)) {
		return satSolverLiteralListRef{}, arena.ErrSlotLimit
	}
	i := uint32(len(s.records))
	s.records = append(s.records, satSolverLiteralListRecord{generation: 1, live: true})
	return satSolverLiteralListRef{s.origin, i, 1}, nil
}

func (s *satSolverLiteralStore) Size(r satSolverLiteralListRef) (uint32, error) {
	x, e := s.record(r)
	if e != nil {
		return 0, e
	}
	return uint32(len(x.values())), nil
}
func (s *satSolverLiteralStore) Read(r satSolverLiteralListRef, i uint32) (Literal, error) {
	x, e := s.record(r)
	if e != nil {
		return 0, e
	}
	if e = satWatchLogicalBounds(i, len(x.values())); e != nil {
		return 0, e
	}
	return x.backing.data[i], nil
}
func (s *satSolverLiteralStore) Write(r satSolverLiteralListRef, i uint32, l Literal) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	if e = satWatchLogicalBounds(i, len(x.values())); e != nil {
		return e
	}
	x.backing.data[i] = l
	return nil
}
func (s *satSolverLiteralStore) Push(r satSolverLiteralListRef, l Literal) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	old := x.values()
	if uint64(len(old)) >= uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	next := append(old, l)
	if x.backing == nil || cap(old) != cap(next) {
		if x.backing != nil {
			x.backing.alive = false
			x.backing.data = nil
		}
		x.backing = &satLiteralBacking{data: next, alive: true}
	} else {
		x.backing.data = next
	}
	return nil
}
func (s *satSolverLiteralStore) Shrink(r satSolverLiteralListRef, n uint32) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	data := x.values()
	if uint64(n) > uint64(len(data)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(data[n:])
	if x.backing != nil {
		x.backing.data = data[:n]
	}
	return nil
}
func (s *satSolverLiteralStore) CloneList(r satSolverLiteralListRef) (satSolverLiteralListRef, error) {
	return s.CopyList(s, r)
}
func (s *satSolverLiteralStore) CopyList(source *satSolverLiteralStore, r satSolverLiteralListRef) (satSolverLiteralListRef, error) {
	if e := s.open(); e != nil {
		return satSolverLiteralListRef{}, e
	}
	x, e := source.record(r)
	if e != nil {
		return satSolverLiteralListRef{}, e
	}
	data := append([]Literal(nil), x.values()...)
	result, e := s.NewList()
	if e != nil {
		return satSolverLiteralListRef{}, e
	}
	out, _ := s.record(result)
	if len(data) != 0 {
		out.backing = &satLiteralBacking{data: data, alive: true}
	}
	return result, nil
}
func (s *satSolverLiteralStore) Free(r satSolverLiteralListRef) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	if x.backing != nil {
		x.backing.alive = false
		x.backing.data = nil
		x.backing = nil
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

func (s *satSolverLiteralStore) Close() error {
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
