package sat

// Source: src/sat/sat_var_queue.h:31-67; src/util/heap.h; src/sat/sat_simplifier.cpp:875–919 and src/sat/sat_solver.h case_split_queue.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Concrete priority queue for solver activity and simplifier literal weights.
// Both the heap values and reverse indexes use native Go backing.
type satVarQueue struct {
	literalSource   *satSimplifier
	self            *satVarQueue
	solverActivity  *satSolverActivity
	values, indices []int32
	budget          *arena.Budget
	debug           *diagnostic.DebugContext
	ownerBytes      uint64
	closed          bool
}
type satVarQueueStats struct{ OwnerBytes uint64 }

func (q *satVarQueue) initialize() {
	q.values = []int32{-1}
	q.indices = make([]int32, 128)
}
func newSATVarQueue(b *arena.Budget) (*satVarQueue, error) {
	return newSATVarQueueWithDebug(b, nil)
}

func newSATVarQueueWithDebug(b *arena.Budget, debug *diagnostic.DebugContext) (*satVarQueue, error) {
	n := uint64(reflect.TypeOf(satVarQueue{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	q := &satVarQueue{budget: b, debug: debug, ownerBytes: n}
	q.self = q
	return q, nil
}

func (q *satVarQueue) sourceAssertionFailure(line int32, condition string) error {
	if e := q.debug.NotifyAssertionViolation("src/util/heap.h", line, condition); e != nil {
		return e
	}
	return q.debug.InvokeAssertionDebugger()
}

func (q *satVarQueue) heapAssertionEnabled() (bool, error) {
	if q.debug == nil {
		return false, nil
	}
	on, e := diagnostic.SourceAssertionsEnabled(q.debug)
	if e != nil || !on {
		return false, e
	}
	return q.debug.IsDebugEnabled("heap")
}

func (q *satVarQueue) cassertInvariant() error {
	on, e := q.heapAssertionEnabled()
	if e != nil || !on {
		return e
	}
	// The source outer check_invariant returns true after nested SASSERTs.
	var walk func(int) error
	walk = func(i int) error {
		if i >= len(q.values) {
			return nil
		}
		v := q.values[i]
		if v < 0 || int64(v) >= int64(len(q.indices)) || q.indices[v] != int32(i) {
			if e := q.sourceAssertionFailure(63, "m_value2indices[m_values[idx]] == idx"); e != nil {
				return e
			}
		}
		if i > 1 {
			less, e := q.lt(v, q.values[i/2])
			if e != nil {
				return e
			}
			if less {
				if e := q.sourceAssertionFailure(64, "parent(idx) == 0 || !less_than(m_values[idx], m_values[parent(idx)])"); e != nil {
					return e
				}
			}
		}
		if e := walk(i * 2); e != nil {
			return e
		}
		return walk(i*2 + 1)
	}
	return walk(1)
}

func (q *satVarQueue) cassertNotContains(v BoolVar) error {
	on, e := q.heapAssertionEnabled()
	if e != nil || !on || !q.Contains(v) {
		return e
	}
	return q.sourceAssertionFailure(232, "!contains(val)")
}
func (q *satVarQueue) open() error {
	if q == nil || q.self != q {
		return ErrSATOwner
	}

	if q.closed {
		return arena.ErrClosed
	}
	return nil
}
func (q *satVarQueue) reserve(n uint32) error {
	if uint64(n) > uint64(^uint32(0)>>1) {
		return arena.ErrBounds
	}
	if uint64(n) <= uint64(len(q.indices)) {
		return q.cassertInvariant()
	}
	q.indices = append(q.indices, make([]int32, int(n)-len(q.indices))...)
	return q.cassertInvariant()
}

func (q *satVarQueue) Size() uint32 {
	if q == nil || q.closed || len(q.values) == 0 {
		return 0
	}
	return uint32(len(q.values) - 1)
}
func (q *satVarQueue) Empty() bool {
	return q != nil && !q.closed && len(q.values) == 1
}
func (q *satVarQueue) Contains(v BoolVar) bool {
	return q != nil && !q.closed && uint64(v) < uint64(len(q.indices)) && q.indices[v] != 0
}
func (q *satVarQueue) lt(a, b int32) (bool, error) {
	if a < 0 || b < 0 {
		return false, arena.ErrBounds
	}
	if q.literalSource != nil {
		x, e := q.literalSource.literalWeight(uint32(a))
		if e != nil {
			return false, e
		}
		y, e := q.literalSource.literalWeight(uint32(b))
		if e != nil {
			return false, e
		}
		return x < y, nil
	}
	x, e := q.getActivity(uint32(a))
	if e != nil {
		return false, e
	}
	y, e := q.getActivity(uint32(b))
	if e != nil {
		return false, e
	}
	return x > y, nil
}
func (q *satVarQueue) put(i int32, v int32) error {
	if i < 0 || int64(i) >= int64(len(q.values)) {
		return arena.ErrBounds
	}
	q.values[i] = v
	if v < 0 || int64(v) >= int64(len(q.indices)) {
		return arena.ErrBounds
	}
	q.indices[v] = i
	return nil
}
func (q *satVarQueue) up(i int32) error {
	if i < 1 || int64(i) >= int64(len(q.values)) {
		return arena.ErrBounds
	}
	v := q.values[i]
	for {
		p := i / 2
		if p == 0 {
			break
		}
		less, e := q.lt(v, q.values[p])
		if e != nil {
			return e
		}
		if !less {
			break
		}
		if e = q.put(i, q.values[p]); e != nil {
			return e
		}
		i = p
	}
	if e := q.put(i, v); e != nil {
		return e
	}
	return q.cassertInvariant()
}
func (q *satVarQueue) down(i int32) error {
	if i < 1 || int64(i) >= int64(len(q.values)) {
		return arena.ErrBounds
	}
	v := q.values[i]
	for {
		left := uint64(i) * 2
		if left >= uint64(len(q.values)) {
			break
		}
		c := int32(left)
		if left+1 < uint64(len(q.values)) {
			less, e := q.lt(q.values[c+1], q.values[c])
			if e != nil {
				return e
			}
			if less {
				c++
			}
		}
		less, e := q.lt(q.values[c], v)
		if e != nil {
			return e
		}
		if !less {
			break
		}
		if e = q.put(i, q.values[c]); e != nil {
			return e
		}
		i = c
	}
	if e := q.put(i, v); e != nil {
		return e
	}
	return q.cassertInvariant()
}
func (q *satVarQueue) insert(v BoolVar) error {
	if e := q.open(); e != nil {
		return e
	}
	if e := q.cassertInvariant(); e != nil {
		return e
	}
	if e := q.cassertNotContains(v); e != nil {
		return e
	}
	if uint64(v) >= uint64(len(q.indices)) {
		return arena.ErrBounds
	}
	if len(q.values) >= int(^uint32(0)>>1) {
		return arena.ErrBounds
	}
	i := int32(len(q.values))
	q.indices[v] = i

	q.values = append(q.values, int32(v))
	if e := q.up(i); e != nil {
		return e
	}
	return q.cassertInvariant()
}
func (q *satVarQueue) MkVar(v BoolVar) error {
	if e := q.open(); e != nil {
		return e
	}
	if e := q.reserve(uint32(v) + 1); e != nil {
		return e
	}
	return q.insert(v)
}
func (q *satVarQueue) UnassignVar(v BoolVar) error {
	if e := q.open(); e != nil {
		return e
	}
	if uint64(v) > uint64(^uint32(0)>>1) {
		return arena.ErrBounds
	}
	if !q.Contains(v) {
		return q.insert(v)
	}
	return nil
}
func (q *satVarQueue) ActivityIncreased(v BoolVar) error {
	if e := q.open(); e != nil {
		return e
	}
	if uint64(v) > uint64(^uint32(0)>>1) {
		return arena.ErrBounds
	}
	if q.Contains(v) {
		return q.up(q.indices[v])
	}
	return nil
}
func (q *satVarQueue) DelVar(v BoolVar) error {
	if e := q.open(); e != nil {
		return e
	}
	if uint64(v) > uint64(^uint32(0)>>1) {
		return arena.ErrBounds
	}
	if !q.Contains(v) {
		return nil
	}
	if e := q.cassertInvariant(); e != nil {
		return e
	}
	i := q.indices[v]
	if i < 1 || int64(i) >= int64(len(q.values)) {
		return arena.ErrBounds
	}
	lastPos := len(q.values) - 1
	if int(i) == lastPos {
		q.indices[v] = 0
		q.values = q.values[:lastPos]
	} else {
		last := q.values[lastPos]
		if e := q.put(i, last); e != nil {
			return e
		}
		q.indices[v] = 0
		q.values = q.values[:lastPos]
		p := i / 2
		if p != 0 {
			less, e := q.lt(last, q.values[p])
			if e != nil {
				return e
			}
			if less {
				if e := q.up(i); e != nil {
					return e
				}
				return q.cassertInvariant()
			}
		}
		if e := q.down(i); e != nil {
			return e
		}
	}
	return q.cassertInvariant()
}
func (q *satVarQueue) NextVar() (BoolVar, error) {
	if e := q.open(); e != nil {
		return 0, e
	}
	if e := q.cassertInvariant(); e != nil {
		return 0, e
	}
	if len(q.values) < 2 {
		if satStorageSourceDebug {
			return 0, ErrSATAssertion
		}
		return 0, arena.ErrBounds
	}
	v := q.values[1]
	if v < 0 || int64(v) >= int64(len(q.indices)) {
		return 0, arena.ErrBounds
	}
	if len(q.values) == 2 {
		q.indices[v] = 0
		q.values = q.values[:1]
	} else {
		last := q.values[len(q.values)-1]
		if e := q.put(1, last); e != nil {
			return 0, e
		}
		q.indices[v] = 0
		q.values = q.values[:len(q.values)-1]
		if e := q.down(1); e != nil {
			return BoolVar(v), e
		}
	}

	return BoolVar(v), q.cassertInvariant()
}
func (q *satVarQueue) Reset() error {
	if e := q.open(); e != nil {
		return e
	}
	if e := q.cassertInvariant(); e != nil {
		return e
	}
	if q.Empty() {
		return nil
	}
	clear(q.indices)
	q.values = q.values[:1]
	q.values[0] = -1

	return q.cassertInvariant()
}
func (q *satVarQueue) CheckInvariant() (bool, error) {
	if e := q.open(); e != nil {
		return false, e
	}
	if !satStorageSourceDebug {
		return true, nil
	}
	for i := 1; i < len(q.values); i++ {
		v := q.values[i]
		if v < 0 || int64(v) >= int64(len(q.indices)) || q.indices[v] != int32(i) {
			return false, ErrSATAssertion
		}
		if i > 1 {
			less, e := q.lt(v, q.values[i/2])
			if e != nil {
				return false, e
			}
			if less {
				return false, ErrSATAssertion
			}
		}
	}
	return true, nil
}
func (q *satVarQueue) Stats() satVarQueueStats {
	return satVarQueueStats{OwnerBytes: q.ownerBytes}
}
func (q *satVarQueue) Close() error {
	if q == nil || q.self != q {
		return ErrSATOwner
	}
	if q.closed {
		return nil
	}
	q.budget.Release(q.ownerBytes)
	q.ownerBytes = 0
	q.values = nil
	q.indices = nil
	q.closed = true
	return nil
}
func (q *satVarQueue) getActivity(i uint32) (uint32, error) {
	return q.solverActivity.Get(i)
}
func newSATSolverVarQueue(a *satSolverActivity, b *arena.Budget) (*satVarQueue, error) {
	return newSATSolverVarQueueWithDebug(a, b, nil)
}

func newSATSolverVarQueueWithDebug(a *satSolverActivity, b *arena.Budget, debug *diagnostic.DebugContext) (*satVarQueue, error) {
	if e := a.open(); e != nil {
		return nil, e
	}
	if b == nil || a.budget != b {
		return nil, ErrSATOwner
	}
	q, e := newSATVarQueueWithDebug(b, debug)
	if e != nil {
		return nil, e
	}
	q.solverActivity = a
	q.initialize()
	if e := q.cassertInvariant(); e != nil {
		_ = q.Close()
		return nil, e
	}
	if e := q.cassertInvariant(); e != nil {
		_ = q.Close()
		return nil, e
	}
	return q, nil
}

func (q *satVarQueue) sizeChecked() (uint32, error) {
	if e := q.open(); e != nil {
		return 0, e
	}
	return q.Size(), nil
}
func (q *satVarQueue) emptyChecked() (bool, error) {
	if e := q.open(); e != nil {
		return false, e
	}
	return q.Empty(), nil
}
func (q *satVarQueue) containsChecked(v BoolVar) (bool, error) {
	if e := q.open(); e != nil {
		return false, e
	}
	if uint64(v) > uint64(^uint32(0)>>1) {
		return false, arena.ErrBounds
	}
	return q.Contains(v), nil
}
