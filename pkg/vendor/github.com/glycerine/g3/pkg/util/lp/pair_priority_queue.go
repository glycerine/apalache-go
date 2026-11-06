package lp

// Source: src/util/lp/binary_heap_upair_queue.h/.hpp, actual int/unsigned instances.

// IntPairPriorityQueue stores ordered index pairs with native map/slice bookkeeping.
// Slots are reused in LIFO order. Resize changes the underlying queue extent
// only, as in the source; it does not change the pair directory. Do not copy.
type IntPairPriorityQueue struct {
	pairQueueIndex
	queue *IntPriorityQueue
}

func NewIntPairPriorityQueue(n uint32) *IntPairPriorityQueue {
	return &IntPairPriorityQueue{pairQueueIndex: newPairQueueIndex(n), queue: NewIntPriorityQueue(n)}
}
func (q *IntPairPriorityQueue) Size() uint32    { return q.queue.Size() }
func (q *IntPairPriorityQueue) IsEmpty() bool   { return q.queue.IsEmpty() }
func (q *IntPairPriorityQueue) Resize(n uint32) { q.queue.Resize(n) }
func (q *IntPairPriorityQueue) Enqueue(i, j uint32, priority int32) error {
	slot, err := q.slot(IndexPair{i, j})
	if err != nil {
		return err
	}
	return q.queue.Enqueue(slot, priority)
}
func (q *IntPairPriorityQueue) Remove(i, j uint32) error {
	pair := IndexPair{i, j}
	slot, ok := q.index[pair]
	if !ok {
		return nil
	}
	if err := q.queue.Remove(slot); err != nil {
		return err
	}
	q.available = append(q.available, slot)
	delete(q.index, pair)
	return nil
}
func (q *IntPairPriorityQueue) Dequeue() (IndexPair, error) {
	if q.queue.IsEmpty() {
		if err := sourceAssertionFailure(); err != nil {
			return IndexPair{}, err
		}
	}
	slot, err := q.queue.Dequeue()
	if err != nil {
		return IndexPair{}, err
	}
	return q.release(slot)
}
func (q *IntPairPriorityQueue) Priority(i, j uint32) (int32, error) {
	slot, ok := q.index[IndexPair{i, j}]
	if !ok {
		return 0xFFFFFF, nil
	}
	return q.queue.Priority(slot)
}
func (q *IntPairPriorityQueue) IsCorrect() (bool, error) {
	consistent, err := q.queue.IsConsistent()
	if err != nil || !consistent {
		return false, err
	}
	return q.bijective() && q.availableCorrect(), nil
}

// UintPairPriorityQueue stores ordered index pairs with native map/slice bookkeeping.
// Slots are reused in LIFO order. Resize changes the underlying queue extent
// only, as in the source; it does not change the pair directory. Do not copy.
type UintPairPriorityQueue struct {
	pairQueueIndex
	queue *UintPriorityQueue
}

func NewUintPairPriorityQueue(n uint32) *UintPairPriorityQueue {
	return &UintPairPriorityQueue{pairQueueIndex: newPairQueueIndex(n), queue: NewUintPriorityQueue(n)}
}
func (q *UintPairPriorityQueue) Size() uint32    { return q.queue.Size() }
func (q *UintPairPriorityQueue) IsEmpty() bool   { return q.queue.IsEmpty() }
func (q *UintPairPriorityQueue) Resize(n uint32) { q.queue.Resize(n) }
func (q *UintPairPriorityQueue) Enqueue(i, j uint32, priority uint32) error {
	slot, err := q.slot(IndexPair{i, j})
	if err != nil {
		return err
	}
	return q.queue.Enqueue(slot, priority)
}
func (q *UintPairPriorityQueue) Remove(i, j uint32) error {
	pair := IndexPair{i, j}
	slot, ok := q.index[pair]
	if !ok {
		return nil
	}
	if err := q.queue.Remove(slot); err != nil {
		return err
	}
	q.available = append(q.available, slot)
	delete(q.index, pair)
	return nil
}
func (q *UintPairPriorityQueue) Dequeue() (IndexPair, error) {
	if q.queue.IsEmpty() {
		if err := sourceAssertionFailure(); err != nil {
			return IndexPair{}, err
		}
	}
	slot, err := q.queue.Dequeue()
	if err != nil {
		return IndexPair{}, err
	}
	return q.release(slot)
}
func (q *UintPairPriorityQueue) Priority(i, j uint32) (uint32, error) {
	slot, ok := q.index[IndexPair{i, j}]
	if !ok {
		return 0xFFFFFF, nil
	}
	return q.queue.Priority(slot)
}
func (q *UintPairPriorityQueue) IsCorrect() (bool, error) {
	consistent, err := q.queue.IsConsistent()
	if err != nil || !consistent {
		return false, err
	}
	return q.bijective() && q.availableCorrect(), nil
}
