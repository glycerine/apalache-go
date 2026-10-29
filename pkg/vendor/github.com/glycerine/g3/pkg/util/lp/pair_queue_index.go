package lp

// Source: src/util/lp/binary_heap_upair_queue.h/.hpp, concrete pair bookkeeping.

// IndexPair identifies a matrix row/column pair. Pair priority queues distinguish
// (i,j) from (j,i); their map has no iteration-order contract.
type IndexPair struct{ I, J uint32 }
type pairQueueIndex struct {
	pairs     []IndexPair
	index     map[IndexPair]uint32
	available []uint32
}

func newPairQueueIndex(n uint32) pairQueueIndex {
	s := pairQueueIndex{pairs: make([]IndexPair, n), index: make(map[IndexPair]uint32), available: make([]uint32, n)}
	for i := range s.available {
		s.available[i] = uint32(i)
	}
	return s
}
func (s *pairQueueIndex) Contains(i, j uint32) bool { _, ok := s.index[IndexPair{i, j}]; return ok }
func (s *pairQueueIndex) dequeueAvailableSpot() (uint32, error) {
	if len(s.available) == 0 {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
		return 0, ErrBounds
	}
	last := len(s.available) - 1
	value := s.available[last]
	s.available = s.available[:last]
	return value, nil
}
func (s *pairQueueIndex) slot(p IndexPair) (uint32, error) {
	if index, ok := s.index[p]; ok {
		return index, nil
	}
	if len(s.available) == 0 {
		old := uint32(len(s.pairs))
		n := old << 1
		for i := old; i < n; i++ {
			s.available = append(s.available, i)
		}
		if uint64(n) <= uint64(len(s.pairs)) {
			s.pairs = s.pairs[:n]
		} else {
			s.pairs = append(s.pairs, make([]IndexPair, int(n)-len(s.pairs))...)
		}
	}
	index, err := s.dequeueAvailableSpot()
	if err != nil {
		return 0, err
	}
	if uint64(index) >= uint64(len(s.pairs)) {
		return 0, ErrBounds
	}
	s.pairs[index] = p
	s.index[p] = index
	return index, nil
}
func (s *pairQueueIndex) release(index uint32) (IndexPair, error) {
	if uint64(index) >= uint64(len(s.pairs)) {
		return IndexPair{}, ErrBounds
	}
	p := s.pairs[index]
	s.available = append(s.available, index)
	delete(s.index, p)
	return p, nil
}
func (s *pairQueueIndex) bijective() bool {
	seen := make(map[uint32]bool, len(s.index))
	for _, index := range s.index {
		if seen[index] {
			return false
		}
		seen[index] = true
	}
	return true
}
func (s *pairQueueIndex) availableCorrect() bool {
	seen := make(map[uint32]bool, len(s.available))
	for _, index := range s.available {
		seen[index] = true
	}
	if len(seen) != len(s.available) {
		return false
	}
	for _, index := range s.index {
		if seen[index] {
			return false
		}
	}
	return true
}
