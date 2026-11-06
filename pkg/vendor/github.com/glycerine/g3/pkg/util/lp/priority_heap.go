package lp

// Source: src/util/lp/binary_heap_priority_queue.h/.hpp, concrete index storage.

// priorityHeap contains only the shared unsigned/signed index operations. Each
// actual priority type implements its own comparisons; no algebra callback or
// any-valued priority storage is used. Heap positions start at one. Resizing
// preserves size and retained entries literally, even after an invalid shrink.
type priorityHeap struct {
	heap    []uint32
	inverse []int32
	size    uint32
}

func newPriorityHeap(n uint32) priorityHeap {
	h := priorityHeap{}
	h.resize(n)
	return h
}
func (h *priorityHeap) resize(n uint32) {
	if uint64(n)+1 <= uint64(len(h.heap)) {
		h.heap = h.heap[:uint64(n)+1]
	} else {
		h.heap = append(h.heap, make([]uint32, uint64(n)+1-uint64(len(h.heap)))...)
	}
	old := len(h.inverse)
	if uint64(n) <= uint64(old) {
		h.inverse = h.inverse[:n]
	} else {
		h.inverse = append(h.inverse, make([]int32, int(n)-old)...)
		for i := old; i < len(h.inverse); i++ {
			h.inverse[i] = -1
		}
	}
}
func (h *priorityHeap) Size() uint32  { return h.size }
func (h *priorityHeap) IsEmpty() bool { return h.size == 0 }
func (h *priorityHeap) at(i uint32) (uint32, error) {
	if uint64(i) >= uint64(len(h.heap)) {
		return 0, ErrBounds
	}
	return h.heap[i], nil
}
func (h *priorityHeap) position(o uint32) (int32, error) {
	if uint64(o) >= uint64(len(h.inverse)) {
		return 0, ErrBounds
	}
	return h.inverse[o], nil
}
func (h *priorityHeap) putAt(i, o uint32) error {
	if uint64(i) >= uint64(len(h.heap)) {
		return ErrBounds
	}
	h.heap[i] = o
	if uint64(o) >= uint64(len(h.inverse)) {
		return ErrBounds
	}
	h.inverse[o] = int32(i)
	return nil
}
func (h *priorityHeap) swapWithParent(i uint32) error {
	parent, err := h.at(i >> 1)
	if err != nil {
		return err
	}
	child, err := h.at(i)
	if err != nil {
		return err
	}
	if err := h.putAt(i>>1, child); err != nil {
		return err
	}
	return h.putAt(i, parent)
}
func (h *priorityHeap) Clear() error {
	for i := uint32(0); i < h.size; i++ {
		o, err := h.at(i + 1)
		if err != nil {
			return err
		}
		if uint64(o) >= uint64(len(h.inverse)) {
			return ErrBounds
		}
		h.inverse[o] = -1
	}
	h.size = 0
	return nil
}
func (h *priorityHeap) checkInverse() error {
	for o, pos := range h.inverse {
		if int64(pos) > int64(h.size) {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
		}
		if pos == -1 {
			continue
		}
		if pos < 0 || uint64(pos) >= uint64(len(h.heap)) {
			return ErrBounds
		}
		if h.heap[pos] != uint32(o) {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
		}
	}
	return nil
}
