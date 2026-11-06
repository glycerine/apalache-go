package lp

// Source: src/util/lp/int_set.h.

import (
	"errors"
	"io"
	"math"
	"strconv"
)

var ErrBounds = errors.New("lp: source index or extent outside supported bounds")

// IntSet keeps the source dense member list and inverse positions. Erasing a
// member moves the last member into its place; iteration is not sorted.
// The zero value is the source empty constructor. Do not copy after use.
type IntSet struct {
	positions []int32
	index     []uint32
}

func NewIntSet(size uint32) (*IntSet, error) {
	s := &IntSet{}
	if err := s.Resize(size); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *IntSet) Contains(j uint32) bool {
	return uint64(j) < uint64(len(s.positions)) && s.positions[j] >= 0
}

func (s *IntSet) Insert(j uint32) error {
	if uint64(j) >= uint64(len(s.positions)) {
		return ErrBounds
	}
	if !s.Contains(j) {
		s.positions[j] = int32(len(s.index))
		s.index = append(s.index, j)
	}
	return nil
}

func (s *IntSet) Erase(j uint32) {
	if !s.Contains(j) {
		return
	}
	pos, last := s.positions[j], len(s.index)-1
	lastJ := s.index[last]
	if int(pos) != last {
		s.positions[lastJ] = pos
		s.index[pos] = lastJ
	}
	s.index = s.index[:last]
	s.positions[j] = -1
}

// Resize changes the domain only, as in the source. Callers must remove members
// outside a shrinking domain first; this method does not silently prune them.
func (s *IntSet) Resize(size uint32) error {
	if size > math.MaxInt32 || uint64(size) > uint64(^uint(0)>>1) {
		return ErrBounds
	}
	old := len(s.positions)
	if int(size) <= old {
		s.positions = s.positions[:size]
		return nil
	}
	s.positions = append(s.positions, make([]int32, int(size)-old)...)
	for i := old; i < len(s.positions); i++ {
		s.positions[i] = -1
	}
	return nil
}

func (s *IntSet) IncreaseSizeByOne() error { return s.Resize(s.DataSize() + 1) }
func (s *IntSet) DataSize() uint32         { return uint32(len(s.positions)) }
func (s *IntSet) Size() uint32             { return uint32(len(s.index)) }
func (s *IntSet) Empty() bool              { return len(s.index) == 0 }

// Indices borrows the actual dense member list. Callers must not modify it.
func (s *IntSet) Indices() []uint32 { return s.index }

func (s *IntSet) Clear() error {
	for _, j := range s.index {
		if uint64(j) >= uint64(len(s.positions)) {
			return ErrBounds
		}
		s.positions[j] = -1
	}
	s.index = s.index[:0]
	return nil
}

// Print uses Go formatting for the source sequence of unsigned/space/newline
// insertions, followed by the source endl flush. File ownership is borrowed.
func (s *IntSet) Print(out io.Writer) error {
	for _, j := range s.index {
		if err := writeConstraintText(out, strconv.FormatUint(uint64(j), 10)+" "); err != nil {
			return err
		}
	}
	return writeDriverLine(out, "")
}
