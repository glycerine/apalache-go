package sat

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// satClauseSet keeps the source swap-erase order while using Go collections for
// the ID index and clause sequence. Clause references are borrowed from owner.
type satClauseSet struct {
	self              *satClauseSet
	owner             *ClauseAllocator
	positions         map[uint32]uint32
	refs              []StoredClauseRef
	budget            *arena.Budget
	ownerBytes, epoch uint64
	closed            bool
}

func NewSATClauseSet(a *ClauseAllocator, b *arena.Budget) (*satClauseSet, error) {
	if e := a.open(); e != nil {
		return nil, e
	}
	return newSATClauseSet(a, b)
}
func newSATClauseSet(a *ClauseAllocator, b *arena.Budget) (*satClauseSet, error) {
	n := uint64(reflect.TypeOf(satClauseSet{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &satClauseSet{owner: a, budget: b, ownerBytes: n, epoch: 1}
	s.self = s
	return s, nil
}
func (s *satClauseSet) open() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *satClauseSet) change() error {
	if e := s.open(); e != nil {
		return e
	}
	if s.epoch == ^uint64(0) {
		return arena.ErrSlotLimit
	}
	return nil
}
func (s *satClauseSet) Size() uint32 {
	if s == nil || s.closed {
		return 0
	}
	return uint32(len(s.refs))
}
func (s *satClauseSet) Empty() bool { return s.Size() == 0 }
func (s *satClauseSet) Contains(r StoredClauseRef) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	id, e := s.owner.ID(r)
	if e != nil {
		return false, e
	}
	_, ok := s.positions[id]
	return ok, nil
}
func (s *satClauseSet) Insert(r StoredClauseRef) error {
	if e := s.change(); e != nil {
		return e
	}
	id, e := s.owner.ID(r)
	if e != nil {
		return e
	}
	if _, ok := s.positions[id]; ok {
		return nil
	}
	if uint64(len(s.refs)) >= uint64(^uint32(0)) {
		return arena.ErrSlotLimit
	}
	if s.positions == nil {
		s.positions = make(map[uint32]uint32)
	}
	s.positions[id] = uint32(len(s.refs))
	s.refs = append(s.refs, r)
	s.epoch++
	return nil
}
func (s *satClauseSet) Erase(r StoredClauseRef) error {
	if e := s.change(); e != nil {
		return e
	}
	id, e := s.owner.ID(r)
	if e != nil {
		return e
	}
	pos, ok := s.positions[id]
	if !ok {
		return nil
	}
	if uint64(pos) >= uint64(len(s.refs)) {
		return arena.ErrBounds
	}
	last := len(s.refs) - 1
	if int(pos) != last {
		lastRef := s.refs[last]
		lid, e := s.owner.ID(lastRef)
		if e != nil {
			return e
		}
		s.refs[pos] = lastRef
		s.positions[lid] = pos
	}
	delete(s.positions, id)
	s.refs[last] = StoredClauseRef{}
	s.refs = s.refs[:last]
	s.epoch++
	return nil
}
func (s *satClauseSet) Pop() (StoredClauseRef, error) {
	if e := s.change(); e != nil {
		return StoredClauseRef{}, e
	}
	if len(s.refs) == 0 {
		if satStorageSourceDebug {
			return StoredClauseRef{}, ErrSATAssertion
		}
		return StoredClauseRef{}, arena.ErrBounds
	}
	last := len(s.refs) - 1
	r := s.refs[last]
	id, e := s.owner.ID(r)
	if e != nil {
		return r, e
	}
	if satStorageSourceDebug {
		if pos, ok := s.positions[id]; !ok || int(pos) != last {
			return r, ErrSATAssertion
		}
	}
	delete(s.positions, id)
	s.refs[last] = StoredClauseRef{}
	s.refs = s.refs[:last]
	s.epoch++
	return r, nil
}
func (s *satClauseSet) Get(i uint32) (StoredClauseRef, error) {
	if e := s.open(); e != nil {
		return StoredClauseRef{}, e
	}
	if uint64(i) >= uint64(len(s.refs)) {
		return StoredClauseRef{}, arena.ErrBounds
	}
	return s.refs[i], nil
}
func (s *satClauseSet) CheckInvariant() (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if !satStorageSourceDebug {
		return true, nil
	}
	if len(s.positions) != len(s.refs) {
		return false, ErrSATAssertion
	}
	for pos, r := range s.refs {
		id, e := s.owner.ID(r)
		if e != nil {
			return false, e
		}
		if got, ok := s.positions[id]; !ok || int(got) != pos {
			return false, ErrSATAssertion
		}
	}
	return true, nil
}
func (s *satClauseSet) Reset() error {
	if e := s.change(); e != nil {
		return e
	}
	clear(s.positions)
	clear(s.refs)
	s.refs = s.refs[:0]
	s.epoch++
	return nil
}
func (s *satClauseSet) Finalize() error {
	if e := s.change(); e != nil {
		return e
	}
	s.positions = nil
	s.refs = nil
	s.epoch++
	return nil
}
func (s *satClauseSet) Close() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return nil
	}
	s.budget.Release(s.ownerBytes)
	s.positions = nil
	s.refs = nil
	s.ownerBytes = 0
	s.closed = true
	return nil
}
func (s *satClauseSet) AppendTo(dst []StoredClauseRef) ([]StoredClauseRef, error) {
	if e := s.open(); e != nil {
		return dst, e
	}
	return append(dst, s.refs...), nil
}
func (s *satClauseSet) Clone(b *arena.Budget) (*satClauseSet, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	v, e := newSATClauseSet(s.owner, b)
	if e != nil {
		return nil, e
	}
	v.refs = append(v.refs, s.refs...)
	if len(s.positions) != 0 {
		v.positions = make(map[uint32]uint32, len(s.positions))
		for id, pos := range s.positions {
			v.positions[id] = pos
		}
	}
	return v, nil
}
func (s *satClauseSet) Assign(src *satClauseSet) error {
	if e := s.change(); e != nil {
		return e
	}
	if e := src.open(); e != nil {
		return e
	}
	if s == src {
		return nil
	}
	if s.owner != src.owner {
		return ErrSATOwner
	}
	if s.positions == nil && len(src.positions) != 0 {
		s.positions = make(map[uint32]uint32, len(src.positions))
	}
	clear(s.positions)
	for id, pos := range src.positions {
		s.positions[id] = pos
	}
	s.refs = append(s.refs[:0], src.refs...)
	s.epoch++
	return nil
}
func (s *satClauseSet) MoveAssign(src *satClauseSet) error { return s.Assign(src) }
func (s *satClauseSet) MoveClone(b *arena.Budget) (*satClauseSet, error) {
	if e := s.change(); e != nil {
		return nil, e
	}
	v, e := newSATClauseSet(s.owner, b)
	if e != nil {
		return nil, e
	}
	v.positions, v.refs = s.positions, s.refs
	s.positions, s.refs = nil, nil
	s.epoch++
	return v, nil
}

type satClauseSetIterator struct {
	owner *satClauseSet
	epoch uint64
	index uint32
}

func (s *satClauseSet) Iter() satClauseSetIterator {
	return satClauseSetIterator{owner: s, epoch: s.epoch}
}
func (i *satClauseSetIterator) Next() (StoredClauseRef, bool, error) {
	if e := i.owner.open(); e != nil {
		return StoredClauseRef{}, false, e
	}
	if i.epoch != i.owner.epoch {
		return StoredClauseRef{}, false, arena.ErrInvalidSlot
	}
	if i.index >= i.owner.Size() {
		return StoredClauseRef{}, false, nil
	}
	r := i.owner.refs[i.index]
	i.index++
	return r, true, nil
}
