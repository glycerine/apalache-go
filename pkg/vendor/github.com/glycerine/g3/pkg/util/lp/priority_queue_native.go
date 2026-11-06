package lp

// Source: src/util/lp/binary_heap_priority_queue.h/.hpp, retained int/unsigned/double instances.

// IntPriorityQueue dequeues the smallest priority first. Construct with NewIntPriorityQueue;
// priorities remain stored after removal/clear, as in the source. Equal priorities
// do not swap. Native slices replace source vector storage. Do not shallow-copy.
type IntPriorityQueue struct {
	priorityHeap
	priorities []int32
}

func NewIntPriorityQueue(n uint32) *IntPriorityQueue {
	return &IntPriorityQueue{priorityHeap: newPriorityHeap(n), priorities: make([]int32, n)}
}
func (q *IntPriorityQueue) Resize(n uint32) {
	if uint64(n) <= uint64(len(q.priorities)) {
		q.priorities = q.priorities[:n]
	} else {
		q.priorities = append(q.priorities, make([]int32, int(n)-len(q.priorities))...)
	}
	q.priorityHeap.resize(n)
}
func (q *IntPriorityQueue) Priority(o uint32) (int32, error) {
	if uint64(o) >= uint64(len(q.priorities)) {
		return 0, ErrBounds
	}
	return q.priorities[o], nil
}
func (q *IntPriorityQueue) heapPriority(i uint32) (int32, error) {
	o, err := q.at(i)
	if err != nil {
		return 0, err
	}
	return q.Priority(o)
}
func (q *IntPriorityQueue) Enqueue(o uint32, priority int32) error {
	if uint64(o) >= uint64(len(q.priorities)) {
		q.Resize(o << 1)
	}
	pos, err := q.position(o)
	if err != nil {
		return err
	}
	if pos == -1 {
		return q.enqueueNew(o, priority)
	}
	return q.changePriority(o, priority)
}
func (q *IntPriorityQueue) enqueueNew(o uint32, priority int32) error {
	q.size++
	i := q.size
	if uint64(o) >= uint64(len(q.priorities)) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	q.priorities[o] = priority
	if err := q.putAt(i, o); err != nil {
		return err
	}
	for i > 1 {
		parent, err := q.heapPriority(i >> 1)
		if err != nil {
			return err
		}
		if !(parent > priority) {
			break
		}
		if err := q.swapWithParent(i); err != nil {
			return err
		}
		i >>= 1
	}
	return nil
}
func (q *IntPriorityQueue) changePriority(o uint32, priority int32) error {
	old, err := q.Priority(o)
	if err != nil {
		return err
	}
	if old > priority {
		q.priorities[o] = priority
		pos, err := q.position(o)
		if err != nil {
			return err
		}
		for pos > 1 {
			child, err := q.heapPriority(uint32(pos))
			if err != nil {
				return err
			}
			parent, err := q.heapPriority(uint32(pos) >> 1)
			if err != nil {
				return err
			}
			if child < parent {
				if err := q.swapWithParent(uint32(pos)); err != nil {
					return err
				}
			} else {
				break
			}
			pos >>= 1
		}
		return nil
	}
	q.priorities[o] = priority
	pos, err := q.position(o)
	if err != nil {
		return err
	}
	return q.fixHeapUnder(uint32(pos))
}
func (q *IntPriorityQueue) fixHeapUnder(i uint32) error {
	for {
		smallest := i
		l := i << 1
		if l <= q.size {
			left, err := q.heapPriority(l)
			if err != nil {
				return err
			}
			parent, err := q.heapPriority(i)
			if err != nil {
				return err
			}
			if left < parent {
				smallest = l
			}
		}
		r := l + 1
		if r <= q.size {
			right, err := q.heapPriority(r)
			if err != nil {
				return err
			}
			best, err := q.heapPriority(smallest)
			if err != nil {
				return err
			}
			if right < best {
				smallest = r
			}
		}
		if smallest == i {
			return nil
		}
		if err := q.swapWithParent(smallest); err != nil {
			return err
		}
		i = smallest
	}
}
func (q *IntPriorityQueue) putLastAtTop() error {
	if q.size > 1 {
		last, err := q.at(q.size)
		if err != nil {
			return err
		}
		q.size--
		if err := q.putAt(1, last); err != nil {
			return err
		}
		return q.fixHeapUnder(1)
	}
	q.size--
	return nil
}
func (q *IntPriorityQueue) Dequeue() (uint32, error) {
	if q.size == 0 {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
	}
	ret, err := q.at(1)
	if err != nil {
		return 0, err
	}
	if err := q.putLastAtTop(); err != nil {
		return 0, err
	}
	if uint64(ret) >= uint64(len(q.inverse)) {
		return 0, ErrBounds
	}
	q.inverse[ret] = -1
	return ret, nil
}
func (q *IntPriorityQueue) IsConsistent() (bool, error) {
	if err := q.checkInverse(); err != nil {
		return false, err
	}
	for i := uint32(1); i < q.size; i++ {
		child := i << 1
		for k := 0; k < 2; k++ {
			if child > q.size {
				break
			}
			parent, err := q.heapPriority(i)
			if err != nil {
				return false, err
			}
			value, err := q.heapPriority(child)
			if err != nil {
				return false, err
			}
			if !(parent <= value) {
				return false, nil
			}
			child++
		}
	}
	return true, nil
}
func (q *IntPriorityQueue) Remove(o uint32) error {
	old, err := q.Priority(o)
	if err != nil {
		return err
	}
	pos, err := q.position(o)
	if err != nil {
		return err
	}
	if pos == -1 {
		return nil
	}
	if uint32(pos) > q.size {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if uint32(pos) < q.size {
		last, err := q.at(q.size)
		if err != nil {
			return err
		}
		q.size--
		if err := q.putAt(uint32(pos), last); err != nil {
			return err
		}
		replacement, err := q.heapPriority(uint32(pos))
		if err != nil {
			return err
		}
		if replacement > old {
			if err := q.fixHeapUnder(uint32(pos)); err != nil {
				return err
			}
		} else {
			i := uint32(pos)
			for i > 1 {
				parent, err := q.heapPriority(i >> 1)
				if err != nil {
					return err
				}
				child, err := q.heapPriority(i)
				if err != nil {
					return err
				}
				if child < parent {
					if err := q.swapWithParent(i); err != nil {
						return err
					}
				} else {
					break
				}
				i >>= 1
			}
		}
	} else {
		if uint32(pos) != q.size {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
		}
		q.size--
	}
	q.inverse[o] = -1
	return nil
}

// UintPriorityQueue dequeues the smallest priority first. Construct with NewUintPriorityQueue;
// priorities remain stored after removal/clear, as in the source. Equal priorities
// do not swap. Native slices replace source vector storage. Do not shallow-copy.
type UintPriorityQueue struct {
	priorityHeap
	priorities []uint32
}

func NewUintPriorityQueue(n uint32) *UintPriorityQueue {
	return &UintPriorityQueue{priorityHeap: newPriorityHeap(n), priorities: make([]uint32, n)}
}
func (q *UintPriorityQueue) Resize(n uint32) {
	if uint64(n) <= uint64(len(q.priorities)) {
		q.priorities = q.priorities[:n]
	} else {
		q.priorities = append(q.priorities, make([]uint32, int(n)-len(q.priorities))...)
	}
	q.priorityHeap.resize(n)
}
func (q *UintPriorityQueue) Priority(o uint32) (uint32, error) {
	if uint64(o) >= uint64(len(q.priorities)) {
		return 0, ErrBounds
	}
	return q.priorities[o], nil
}
func (q *UintPriorityQueue) heapPriority(i uint32) (uint32, error) {
	o, err := q.at(i)
	if err != nil {
		return 0, err
	}
	return q.Priority(o)
}
func (q *UintPriorityQueue) Enqueue(o uint32, priority uint32) error {
	if uint64(o) >= uint64(len(q.priorities)) {
		q.Resize(o << 1)
	}
	pos, err := q.position(o)
	if err != nil {
		return err
	}
	if pos == -1 {
		return q.enqueueNew(o, priority)
	}
	return q.changePriority(o, priority)
}
func (q *UintPriorityQueue) enqueueNew(o uint32, priority uint32) error {
	q.size++
	i := q.size
	if uint64(o) >= uint64(len(q.priorities)) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	q.priorities[o] = priority
	if err := q.putAt(i, o); err != nil {
		return err
	}
	for i > 1 {
		parent, err := q.heapPriority(i >> 1)
		if err != nil {
			return err
		}
		if !(parent > priority) {
			break
		}
		if err := q.swapWithParent(i); err != nil {
			return err
		}
		i >>= 1
	}
	return nil
}
func (q *UintPriorityQueue) changePriority(o uint32, priority uint32) error {
	old, err := q.Priority(o)
	if err != nil {
		return err
	}
	if old > priority {
		q.priorities[o] = priority
		pos, err := q.position(o)
		if err != nil {
			return err
		}
		for pos > 1 {
			child, err := q.heapPriority(uint32(pos))
			if err != nil {
				return err
			}
			parent, err := q.heapPriority(uint32(pos) >> 1)
			if err != nil {
				return err
			}
			if child < parent {
				if err := q.swapWithParent(uint32(pos)); err != nil {
					return err
				}
			} else {
				break
			}
			pos >>= 1
		}
		return nil
	}
	q.priorities[o] = priority
	pos, err := q.position(o)
	if err != nil {
		return err
	}
	return q.fixHeapUnder(uint32(pos))
}
func (q *UintPriorityQueue) fixHeapUnder(i uint32) error {
	for {
		smallest := i
		l := i << 1
		if l <= q.size {
			left, err := q.heapPriority(l)
			if err != nil {
				return err
			}
			parent, err := q.heapPriority(i)
			if err != nil {
				return err
			}
			if left < parent {
				smallest = l
			}
		}
		r := l + 1
		if r <= q.size {
			right, err := q.heapPriority(r)
			if err != nil {
				return err
			}
			best, err := q.heapPriority(smallest)
			if err != nil {
				return err
			}
			if right < best {
				smallest = r
			}
		}
		if smallest == i {
			return nil
		}
		if err := q.swapWithParent(smallest); err != nil {
			return err
		}
		i = smallest
	}
}
func (q *UintPriorityQueue) putLastAtTop() error {
	if q.size > 1 {
		last, err := q.at(q.size)
		if err != nil {
			return err
		}
		q.size--
		if err := q.putAt(1, last); err != nil {
			return err
		}
		return q.fixHeapUnder(1)
	}
	q.size--
	return nil
}
func (q *UintPriorityQueue) Dequeue() (uint32, error) {
	if q.size == 0 {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
	}
	ret, err := q.at(1)
	if err != nil {
		return 0, err
	}
	if err := q.putLastAtTop(); err != nil {
		return 0, err
	}
	if uint64(ret) >= uint64(len(q.inverse)) {
		return 0, ErrBounds
	}
	q.inverse[ret] = -1
	return ret, nil
}
func (q *UintPriorityQueue) IsConsistent() (bool, error) {
	if err := q.checkInverse(); err != nil {
		return false, err
	}
	for i := uint32(1); i < q.size; i++ {
		child := i << 1
		for k := 0; k < 2; k++ {
			if child > q.size {
				break
			}
			parent, err := q.heapPriority(i)
			if err != nil {
				return false, err
			}
			value, err := q.heapPriority(child)
			if err != nil {
				return false, err
			}
			if !(parent <= value) {
				return false, nil
			}
			child++
		}
	}
	return true, nil
}
func (q *UintPriorityQueue) Remove(o uint32) error {
	old, err := q.Priority(o)
	if err != nil {
		return err
	}
	pos, err := q.position(o)
	if err != nil {
		return err
	}
	if pos == -1 {
		return nil
	}
	if uint32(pos) > q.size {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if uint32(pos) < q.size {
		last, err := q.at(q.size)
		if err != nil {
			return err
		}
		q.size--
		if err := q.putAt(uint32(pos), last); err != nil {
			return err
		}
		replacement, err := q.heapPriority(uint32(pos))
		if err != nil {
			return err
		}
		if replacement > old {
			if err := q.fixHeapUnder(uint32(pos)); err != nil {
				return err
			}
		} else {
			i := uint32(pos)
			for i > 1 {
				parent, err := q.heapPriority(i >> 1)
				if err != nil {
					return err
				}
				child, err := q.heapPriority(i)
				if err != nil {
					return err
				}
				if child < parent {
					if err := q.swapWithParent(i); err != nil {
						return err
					}
				} else {
					break
				}
				i >>= 1
			}
		}
	} else {
		if uint32(pos) != q.size {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
		}
		q.size--
	}
	q.inverse[o] = -1
	return nil
}

// FloatPriorityQueue dequeues the smallest priority first. Construct with NewFloatPriorityQueue;
// priorities remain stored after removal/clear, as in the source. Equal priorities
// do not swap. Native slices replace source vector storage. Do not shallow-copy.
type FloatPriorityQueue struct {
	priorityHeap
	priorities []float64
}

func NewFloatPriorityQueue(n uint32) *FloatPriorityQueue {
	return &FloatPriorityQueue{priorityHeap: newPriorityHeap(n), priorities: make([]float64, n)}
}
func (q *FloatPriorityQueue) Resize(n uint32) {
	if uint64(n) <= uint64(len(q.priorities)) {
		q.priorities = q.priorities[:n]
	} else {
		q.priorities = append(q.priorities, make([]float64, int(n)-len(q.priorities))...)
	}
	q.priorityHeap.resize(n)
}
func (q *FloatPriorityQueue) Priority(o uint32) (float64, error) {
	if uint64(o) >= uint64(len(q.priorities)) {
		return 0, ErrBounds
	}
	return q.priorities[o], nil
}
func (q *FloatPriorityQueue) heapPriority(i uint32) (float64, error) {
	o, err := q.at(i)
	if err != nil {
		return 0, err
	}
	return q.Priority(o)
}
func (q *FloatPriorityQueue) Enqueue(o uint32, priority float64) error {
	if uint64(o) >= uint64(len(q.priorities)) {
		q.Resize(o << 1)
	}
	pos, err := q.position(o)
	if err != nil {
		return err
	}
	if pos == -1 {
		return q.enqueueNew(o, priority)
	}
	return q.changePriority(o, priority)
}
func (q *FloatPriorityQueue) enqueueNew(o uint32, priority float64) error {
	q.size++
	i := q.size
	if uint64(o) >= uint64(len(q.priorities)) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	q.priorities[o] = priority
	if err := q.putAt(i, o); err != nil {
		return err
	}
	for i > 1 {
		parent, err := q.heapPriority(i >> 1)
		if err != nil {
			return err
		}
		if !(parent > priority) {
			break
		}
		if err := q.swapWithParent(i); err != nil {
			return err
		}
		i >>= 1
	}
	return nil
}
func (q *FloatPriorityQueue) changePriority(o uint32, priority float64) error {
	old, err := q.Priority(o)
	if err != nil {
		return err
	}
	if old > priority {
		q.priorities[o] = priority
		pos, err := q.position(o)
		if err != nil {
			return err
		}
		for pos > 1 {
			child, err := q.heapPriority(uint32(pos))
			if err != nil {
				return err
			}
			parent, err := q.heapPriority(uint32(pos) >> 1)
			if err != nil {
				return err
			}
			if child < parent {
				if err := q.swapWithParent(uint32(pos)); err != nil {
					return err
				}
			} else {
				break
			}
			pos >>= 1
		}
		return nil
	}
	q.priorities[o] = priority
	pos, err := q.position(o)
	if err != nil {
		return err
	}
	return q.fixHeapUnder(uint32(pos))
}
func (q *FloatPriorityQueue) fixHeapUnder(i uint32) error {
	for {
		smallest := i
		l := i << 1
		if l <= q.size {
			left, err := q.heapPriority(l)
			if err != nil {
				return err
			}
			parent, err := q.heapPriority(i)
			if err != nil {
				return err
			}
			if left < parent {
				smallest = l
			}
		}
		r := l + 1
		if r <= q.size {
			right, err := q.heapPriority(r)
			if err != nil {
				return err
			}
			best, err := q.heapPriority(smallest)
			if err != nil {
				return err
			}
			if right < best {
				smallest = r
			}
		}
		if smallest == i {
			return nil
		}
		if err := q.swapWithParent(smallest); err != nil {
			return err
		}
		i = smallest
	}
}
func (q *FloatPriorityQueue) putLastAtTop() error {
	if q.size > 1 {
		last, err := q.at(q.size)
		if err != nil {
			return err
		}
		q.size--
		if err := q.putAt(1, last); err != nil {
			return err
		}
		return q.fixHeapUnder(1)
	}
	q.size--
	return nil
}
func (q *FloatPriorityQueue) Dequeue() (uint32, error) {
	if q.size == 0 {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
	}
	ret, err := q.at(1)
	if err != nil {
		return 0, err
	}
	if err := q.putLastAtTop(); err != nil {
		return 0, err
	}
	if uint64(ret) >= uint64(len(q.inverse)) {
		return 0, ErrBounds
	}
	q.inverse[ret] = -1
	return ret, nil
}
func (q *FloatPriorityQueue) IsConsistent() (bool, error) {
	if err := q.checkInverse(); err != nil {
		return false, err
	}
	for i := uint32(1); i < q.size; i++ {
		child := i << 1
		for k := 0; k < 2; k++ {
			if child > q.size {
				break
			}
			parent, err := q.heapPriority(i)
			if err != nil {
				return false, err
			}
			value, err := q.heapPriority(child)
			if err != nil {
				return false, err
			}
			if !(parent <= value) {
				return false, nil
			}
			child++
		}
	}
	return true, nil
}
