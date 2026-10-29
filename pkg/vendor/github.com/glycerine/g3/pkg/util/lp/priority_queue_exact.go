package lp

// Source: src/util/lp/binary_heap_priority_queue.h/.hpp, actual mpq and numeric_pair<mpq> consumers.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalPriorityQueue owns concrete exact priorities in a borrowed manager. Removing or
// clearing membership retains priorities; Close destroys them. Decrease owns
// the source's by-value argument copy. After a stopped numeric call, Close is
// the owning continuation. Construct before use; do not shallow-copy.
type RationalPriorityQueue struct {
	priorityHeap
	manager     *numeral.RationalManager
	priorities  *IndexedRationalVector
	decreaseArg *IndexedRationalVector
	calls       exactPriorityCalls
}

func NewRationalPriorityQueue(m *numeral.RationalManager, n uint32) *RationalPriorityQueue {
	return &RationalPriorityQueue{priorityHeap: newPriorityHeap(n), manager: m, priorities: NewIndexedRationalVector(m, n)}
}
func (q *RationalPriorityQueue) ready() error {
	if q.calls.pending() || q.priorities.calls.pending() || q.decreaseArg != nil {
		return ErrExactQueuePending
	}
	return nil
}
func (q *RationalPriorityQueue) Resize(n uint32) error {
	if err := q.ready(); err != nil {
		return err
	}
	return q.resize(n)
}
func (q *RationalPriorityQueue) resize(n uint32) error {
	if err := q.priorities.resizeData(n); err != nil {
		return err
	}
	q.priorityHeap.resize(n)
	return nil
}
func (q *RationalPriorityQueue) Clear() error {
	if err := q.ready(); err != nil {
		return err
	}
	return q.priorityHeap.Clear()
}
func (q *RationalPriorityQueue) priority(o uint32) (*numeral.Rational, error) {
	if uint64(o) >= uint64(len(q.priorities.Data)) {
		return nil, ErrBounds
	}
	return &q.priorities.Data[o], nil
}
func (q *RationalPriorityQueue) heapPriority(i uint32) (*numeral.Rational, error) {
	o, err := q.at(i)
	if err != nil {
		return nil, err
	}
	return q.priority(o)
}
func (q *RationalPriorityQueue) less(a, b *numeral.Rational) (value bool, err error) {
	before, err := q.calls.begin(q.manager)
	if err != nil {
		return false, err
	}
	defer func() { err = q.calls.after(q.manager, before, err) }()
	c, err := q.manager.Compare(a, b)
	return c < 0, err
}
func (q *RationalPriorityQueue) greater(a, b *numeral.Rational) (value bool, err error) {
	before, err := q.calls.begin(q.manager)
	if err != nil {
		return false, err
	}
	defer func() { err = q.calls.after(q.manager, before, err) }()
	c, err := q.manager.Compare(b, a)
	return c < 0, err
}
func (q *RationalPriorityQueue) Enqueue(o uint32, priority *numeral.Rational) error {
	if err := q.ready(); err != nil {
		return err
	}
	if uint64(o) >= uint64(len(q.priorities.Data)) {
		if err := q.resize(o << 1); err != nil {
			return err
		}
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
func (q *RationalPriorityQueue) enqueueNew(o uint32, priority *numeral.Rational) error {
	q.size++
	i := q.size
	if uint64(o) >= uint64(len(q.priorities.Data)) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	if err := q.priorities.copySlot(&q.priorities.Data[o], priority); err != nil {
		return err
	}
	if err := q.putAt(i, o); err != nil {
		return err
	}
	for i > 1 {
		parent, err := q.heapPriority(i >> 1)
		if err != nil {
			return err
		}
		greater, err := q.greater(parent, priority)
		if err != nil {
			return err
		}
		if !greater {
			break
		}
		if err := q.swapWithParent(i); err != nil {
			return err
		}
		i >>= 1
	}
	return nil
}
func (q *RationalPriorityQueue) changePriority(o uint32, priority *numeral.Rational) error {
	old, err := q.priority(o)
	if err != nil {
		return err
	}
	greater, err := q.greater(old, priority)
	if err != nil {
		return err
	}
	if greater {
		return q.decreasePriority(o, priority)
	}
	if err := q.priorities.copySlot(old, priority); err != nil {
		return err
	}
	pos, err := q.position(o)
	if err != nil {
		return err
	}
	return q.fixHeapUnder(uint32(pos))
}
func (q *RationalPriorityQueue) decreasePriority(o uint32, priority *numeral.Rational) error {
	// Source decrease_priority takes T by value before entering its body.
	q.decreaseArg = NewIndexedRationalVector(q.manager, 1)
	if err := q.decreaseArg.copySlot(&q.decreaseArg.Data[0], priority); err != nil {
		return err
	}
	old, err := q.priority(o)
	if err != nil {
		return err
	}
	if err := q.priorities.copySlot(old, &q.decreaseArg.Data[0]); err != nil {
		return err
	}
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
		less, err := q.less(child, parent)
		if err != nil {
			return err
		}
		if !less {
			break
		}
		if err := q.swapWithParent(uint32(pos)); err != nil {
			return err
		}
		pos >>= 1
	}
	if err := q.decreaseArg.Close(); err != nil {
		return err
	}
	q.decreaseArg = nil
	return nil
}
func (q *RationalPriorityQueue) fixHeapUnder(i uint32) error {
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
			less, err := q.less(left, parent)
			if err != nil {
				return err
			}
			if less {
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
			less, err := q.less(right, best)
			if err != nil {
				return err
			}
			if less {
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
func (q *RationalPriorityQueue) putLastAtTop() error {
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
func (q *RationalPriorityQueue) Dequeue() (uint32, error) {
	if err := q.ready(); err != nil {
		return 0, err
	}
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
func (q *RationalPriorityQueue) Close() error {
	if err := q.calls.cleanupRaw(q.manager); err != nil {
		return err
	}
	if q.decreaseArg != nil {
		if err := q.decreaseArg.Close(); err != nil {
			return err
		}
		q.decreaseArg = nil
	}
	if err := q.priorities.Close(); err != nil {
		return err
	}
	if err := q.calls.cleanupRoots(q.manager); err != nil {
		return err
	}
	q.priorityHeap = priorityHeap{}
	return nil
}

// PairPriorityQueue owns concrete exact priorities in a borrowed manager. Removing or
// clearing membership retains priorities; Close destroys them. Decrease owns
// the source's by-value argument copy. After a stopped numeric call, Close is
// the owning continuation. Construct before use; do not shallow-copy.
type PairPriorityQueue struct {
	priorityHeap
	manager     *numeral.RationalManager
	priorities  *IndexedPairVector
	decreaseArg *IndexedPairVector
	calls       exactPriorityCalls
}

func NewPairPriorityQueue(m *numeral.RationalManager, n uint32) *PairPriorityQueue {
	return &PairPriorityQueue{priorityHeap: newPriorityHeap(n), manager: m, priorities: NewIndexedPairVector(m, n)}
}
func (q *PairPriorityQueue) ready() error {
	if q.calls.pending() || q.priorities.calls.pending() || q.decreaseArg != nil {
		return ErrExactQueuePending
	}
	return nil
}
func (q *PairPriorityQueue) Resize(n uint32) error {
	if err := q.ready(); err != nil {
		return err
	}
	return q.resize(n)
}
func (q *PairPriorityQueue) resize(n uint32) error {
	if err := q.priorities.resizeData(n); err != nil {
		return err
	}
	q.priorityHeap.resize(n)
	return nil
}
func (q *PairPriorityQueue) Clear() error {
	if err := q.ready(); err != nil {
		return err
	}
	return q.priorityHeap.Clear()
}
func (q *PairPriorityQueue) priority(o uint32) (*RationalPair, error) {
	if uint64(o) >= uint64(len(q.priorities.Data)) {
		return nil, ErrBounds
	}
	return &q.priorities.Data[o], nil
}
func (q *PairPriorityQueue) heapPriority(i uint32) (*RationalPair, error) {
	o, err := q.at(i)
	if err != nil {
		return nil, err
	}
	return q.priority(o)
}
func (q *PairPriorityQueue) less(a, b *RationalPair) (value bool, err error) {
	before, err := q.calls.begin(q.manager)
	if err != nil {
		return false, err
	}
	defer func() { err = q.calls.after(q.manager, before, err) }()
	return a.Less(q.manager, b)
}
func (q *PairPriorityQueue) greater(a, b *RationalPair) (value bool, err error) {
	before, err := q.calls.begin(q.manager)
	if err != nil {
		return false, err
	}
	defer func() { err = q.calls.after(q.manager, before, err) }()
	return a.Greater(q.manager, b)
}
func (q *PairPriorityQueue) Enqueue(o uint32, priority *RationalPair) error {
	if err := q.ready(); err != nil {
		return err
	}
	if uint64(o) >= uint64(len(q.priorities.Data)) {
		if err := q.resize(o << 1); err != nil {
			return err
		}
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
func (q *PairPriorityQueue) enqueueNew(o uint32, priority *RationalPair) error {
	q.size++
	i := q.size
	if uint64(o) >= uint64(len(q.priorities.Data)) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	if err := q.priorities.copySlot(&q.priorities.Data[o], priority); err != nil {
		return err
	}
	if err := q.putAt(i, o); err != nil {
		return err
	}
	for i > 1 {
		parent, err := q.heapPriority(i >> 1)
		if err != nil {
			return err
		}
		greater, err := q.greater(parent, priority)
		if err != nil {
			return err
		}
		if !greater {
			break
		}
		if err := q.swapWithParent(i); err != nil {
			return err
		}
		i >>= 1
	}
	return nil
}
func (q *PairPriorityQueue) changePriority(o uint32, priority *RationalPair) error {
	old, err := q.priority(o)
	if err != nil {
		return err
	}
	greater, err := q.greater(old, priority)
	if err != nil {
		return err
	}
	if greater {
		return q.decreasePriority(o, priority)
	}
	if err := q.priorities.copySlot(old, priority); err != nil {
		return err
	}
	pos, err := q.position(o)
	if err != nil {
		return err
	}
	return q.fixHeapUnder(uint32(pos))
}
func (q *PairPriorityQueue) decreasePriority(o uint32, priority *RationalPair) error {
	// Source decrease_priority takes T by value before entering its body.
	q.decreaseArg = NewIndexedPairVector(q.manager, 1)
	if err := q.decreaseArg.copySlot(&q.decreaseArg.Data[0], priority); err != nil {
		return err
	}
	old, err := q.priority(o)
	if err != nil {
		return err
	}
	if err := q.priorities.copySlot(old, &q.decreaseArg.Data[0]); err != nil {
		return err
	}
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
		less, err := q.less(child, parent)
		if err != nil {
			return err
		}
		if !less {
			break
		}
		if err := q.swapWithParent(uint32(pos)); err != nil {
			return err
		}
		pos >>= 1
	}
	if err := q.decreaseArg.Close(); err != nil {
		return err
	}
	q.decreaseArg = nil
	return nil
}
func (q *PairPriorityQueue) fixHeapUnder(i uint32) error {
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
			less, err := q.less(left, parent)
			if err != nil {
				return err
			}
			if less {
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
			less, err := q.less(right, best)
			if err != nil {
				return err
			}
			if less {
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
func (q *PairPriorityQueue) putLastAtTop() error {
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
func (q *PairPriorityQueue) Dequeue() (uint32, error) {
	if err := q.ready(); err != nil {
		return 0, err
	}
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
func (q *PairPriorityQueue) Close() error {
	if err := q.calls.cleanupRaw(q.manager); err != nil {
		return err
	}
	if q.decreaseArg != nil {
		if err := q.decreaseArg.Close(); err != nil {
			return err
		}
		q.decreaseArg = nil
	}
	if err := q.priorities.Close(); err != nil {
		return err
	}
	if err := q.calls.cleanupRoots(q.manager); err != nil {
		return err
	}
	q.priorityHeap = priorityHeap{}
	return nil
}
