package sat

// Source: src/sat/sat_types.h:153-204,229-263; src/sat/sat_probing.cpp.

import "github.com/glycerine/g3/pkg/arena"

// satLiteralSetStorage holds the concrete literals used by probing,
// assumption tracking, MUS and consequence sets. Values retain first insertion order. Go owns storage.
type satLiteralSetStorage struct {
	membership []bool
	values     []Literal
}

// satLiteralSetIterator is the concrete live specialization of
// literal_set::iterator. Begin captures the source begin/end pair; callers must
// not mutate the set until iteration finishes.
type satLiteralSetIterator struct {
	values []Literal
	next   int
}

func (s *satLiteralSetStorage) Insert(l Literal) error {
	v := l.index()
	// Source v+1 wraps at the invalid all-ones literal index. Guard that
	// source-undefined access before allocating or publishing membership.
	if v == ^uint32(0) {
		return arena.ErrBounds
	}
	n := int(v) + 1
	if n > len(s.membership) {
		s.membership = append(s.membership, make([]bool, n-len(s.membership))...)
	}
	if !s.membership[v] {
		s.membership[v] = true
		s.values = append(s.values, l)
	}
	return nil
}
func (s *satLiteralSetStorage) contains(l Literal) bool {
	return uint64(l.index()) < uint64(len(s.membership)) && s.membership[l.index()]
}
func (s *satLiteralSetStorage) size() uint32 { return uint32(len(s.values)) }

// Remove preserves uint_set::remove's swap-with-last compaction. Consequently
// removal does not preserve insertion order, while all untouched entries keep
// their current relative storage positions.
func (s *satLiteralSetStorage) Remove(l Literal) error {
	if !s.contains(l) {
		return nil
	}
	index := l.index()
	s.membership[index] = false
	for i, value := range s.values {
		if value != l {
			continue
		}
		last := len(s.values) - 1
		s.values[i] = s.values[last]
		s.values[last] = 0
		s.values = s.values[:last]
		return nil
	}
	return ErrSATAssertion
}

// Pop preserves literal_set::pop and uint_set::erase: the last stored literal
// is removed and its membership bit is cleared.
func (s *satLiteralSetStorage) Pop() (Literal, error) {
	if len(s.values) == 0 {
		return 0, arena.ErrBounds
	}
	last := len(s.values) - 1
	literal := s.values[last]
	s.values[last] = 0
	s.values = s.values[:last]
	index := literal.index()
	if uint64(index) >= uint64(len(s.membership)) {
		return literal, arena.ErrBounds
	}
	s.membership[index] = false
	return literal, nil
}

func (s *satLiteralSetStorage) AssignLiterals(values []Literal) error {
	s.Reset()
	for _, literal := range values {
		if err := s.Insert(literal); err != nil {
			return err
		}
	}
	return nil
}

func (s *satLiteralSetStorage) Assign(other *satLiteralSetStorage) {
	if s == other {
		return
	}
	s.membership = append(s.membership[:0], other.membership...)
	s.values = append(s.values[:0], other.values...)
}

func (s *satLiteralSetStorage) ToVector() []Literal {
	return append([]Literal(nil), s.values...)
}

func (s *satLiteralSetStorage) Begin() satLiteralSetIterator {
	return satLiteralSetIterator{values: s.values}
}

func (i *satLiteralSetIterator) Next() (Literal, bool) {
	if i.next >= len(i.values) {
		return 0, false
	}
	literal := i.values[i.next]
	i.next++
	return literal, true
}

func (s *satLiteralSetStorage) Intersect(other *satLiteralSetStorage) {
	if s == other {
		return
	}
	j := 0
	for _, literal := range s.values {
		if other.contains(literal) {
			s.values[j] = literal
			j++
		} else {
			s.membership[literal.index()] = false
		}
	}
	clear(s.values[j:])
	s.values = s.values[:j]
}

func (s *satLiteralSetStorage) Union(other *satLiteralSetStorage) error {
	for _, literal := range other.values {
		if err := s.Insert(literal); err != nil {
			return err
		}
	}
	return nil
}

func (s *satLiteralSetStorage) Reset() {
	s.membership = s.membership[:0]
	s.values = s.values[:0]
}
func (s *satLiteralSetStorage) Finalize() {
	s.membership = nil
	s.values = nil
}
