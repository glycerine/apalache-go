package sat

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *satCutSet) grow() error {
	oldCap := cap(s.cuts)
	newCap := 2
	if oldCap != 0 {
		if oldCap > math.MaxInt/2 {
			return arena.ErrSlotLimit
		}
		newCap = oldCap * 2
	}
	cutSize := uint64(reflect.TypeOf(satCut{}).Size())
	if uint64(newCap) > math.MaxUint64/cutSize {
		return arena.ErrSlotLimit
	}
	newBytes := uint64(newCap) * cutSize
	if err := s.budget.Reserve(newBytes); err != nil {
		return err
	}
	next := make([]satCut, len(s.cuts), newCap)
	copy(next, s.cuts)
	s.budget.Release(s.cutBytes)
	s.cuts, s.cutBytes = next, newBytes
	return nil
}

func (s *satCutSet) PushBack(onAdd satCutUpdate, c satCut) error {
	if err := s.open(); err != nil {
		return err
	}
	if len(s.cuts) == cap(s.cuts) {
		if err := s.grow(); err != nil {
			return err
		}
	}
	// Source publishes proof bookkeeping before publishing the new cut.
	if s.variable != math.MaxUint32 && onAdd != nil {
		onAdd(s.variable, c)
	}
	s.cuts = append(s.cuts, c)
	return nil
}

func (s *satCutSet) Insert(onAdd, onDel satCutUpdate, c satCut) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	// Determine rejection without rearranging the retained antichain. Go's
	// budget failure is recoverable, so unlike region allocation in the source,
	// failed growth must leave the observable set unchanged.
	for i := range s.cuts {
		if s.cuts[i].SubsetOf(c) {
			return false, nil
		}
	}
	if len(s.cuts) == cap(s.cuts) {
		if err := s.grow(); err != nil {
			return false, err
		}
	}
	i, k := 0, len(s.cuts)
	for i < k {
		a := s.cuts[i]
		if c.SubsetOf(a) {
			k--
			s.cuts[i], s.cuts[k] = s.cuts[k], s.cuts[i]
			continue
		}
		i++
	}
	if err := s.PushBack(onAdd, c); err != nil {
		return false, err
	}
	s.cuts[i], s.cuts[len(s.cuts)-1] = s.cuts[len(s.cuts)-1], s.cuts[i]
	i++
	s.shrinkUnchecked(onDel, i)
	return true, nil
}

func (s *satCutSet) NoDuplicates() bool {
	if s == nil || s.closed {
		return false
	}
	for i := range s.cuts {
		for j := 0; j < i; j++ {
			if s.cuts[i].Equal(s.cuts[j]) {
				return false
			}
		}
	}
	return true
}
