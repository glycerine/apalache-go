package sat

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *satCutSet) shrinkUnchecked(onDel satCutUpdate, size int) {
	if s.variable != math.MaxUint32 && onDel != nil {
		for i := size; i < len(s.cuts); i++ {
			onDel(s.variable, s.cuts[i])
		}
	}
	s.cuts = s.cuts[:size]
}

func (s *satCutSet) Shrink(onDel satCutUpdate, size uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if size > uint32(len(s.cuts)) {
		return arena.ErrBounds
	}
	s.shrinkUnchecked(onDel, int(size))
	return nil
}

func (s *satCutSet) Reset(onDel satCutUpdate) error { return s.Shrink(onDel, 0) }

func (s *satCutSet) EvictIndex(onDel satCutUpdate, index uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if index >= uint32(len(s.cuts)) {
		return arena.ErrBounds
	}
	if s.variable != math.MaxUint32 && onDel != nil {
		onDel(s.variable, s.cuts[index])
	}
	last := len(s.cuts) - 1
	s.cuts[index] = s.cuts[last]
	s.cuts = s.cuts[:last]
	return nil
}

func (s *satCutSet) Evict(onDel satCutUpdate, c satCut) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	for i := range s.cuts {
		if s.cuts[i].Equal(c) {
			return true, s.EvictIndex(onDel, uint32(i))
		}
	}
	return false, nil
}

func (s *satCutSet) Swap(other *satCutSet) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if s == other {
		return nil
	}
	// A source region owns both arrays. The concrete Go analogue requires the
	// same budget owner so physical backing charges remain correctly paired.
	if s.budget != other.budget {
		return ErrSATOwner
	}
	s.variable, other.variable = other.variable, s.variable
	s.cuts, other.cuts = other.cuts, s.cuts
	s.cutBytes, other.cutBytes = other.cutBytes, s.cutBytes
	return nil
}

func (s *satCutSet) Close() error {
	if err := s.open(); err != nil {
		return err
	}
	s.budget.Release(s.cutBytes + s.ownerBytes)
	s.cuts, s.cutBytes, s.ownerBytes, s.closed = nil, 0, 0, true
	return nil
}
