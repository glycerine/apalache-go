package euclid

// Source: src/math/euclid/euclidean_solver.cpp:39-45,315-320,452-455;
// src/util/heap.h:57-66,219-250. The retained specialization is local to
// the Euclidean solver; Go's heap package maintains the typed slice order.

import (
	"container/heap"
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var errSolvedQueueValue = errors.New("euclid: invalid or duplicate solved variable")

type solvedQueue struct {
	owner     *solverSetup
	debug     *diagnostic.DebugContext
	values    []int32
	positions map[int32]int
	bounds    int32
	closed    bool
}

func (q *solvedQueue) Len() int { return len(q.values) }
func (q *solvedQueue) Less(i, j int) bool {
	return q.owner.solved[q.values[i]] < q.owner.solved[q.values[j]]
}
func (q *solvedQueue) Swap(i, j int) {
	q.values[i], q.values[j] = q.values[j], q.values[i]
	q.positions[q.values[i]], q.positions[q.values[j]] = i, j
}
func (q *solvedQueue) Push(value any) {
	v := value.(int32)
	q.positions[v] = len(q.values)
	q.values = append(q.values, v)
}
func (q *solvedQueue) Pop() any {
	last := len(q.values) - 1
	v := q.values[last]
	q.values[last] = 0
	q.values = q.values[:last]
	delete(q.positions, v)
	return v
}

func (q *solvedQueue) alive() error {
	if q == nil || q.owner == nil {
		return arena.ErrInvalidSlot
	}
	if q.closed {
		return arena.ErrClosed
	}
	return nil
}
func (q *solvedQueue) debugContext() *diagnostic.DebugContext {
	if q.debug != nil {
		return q.debug
	}
	return diagnostic.BorrowProcessDebugContext()
}
func (q *solvedQueue) assertFailure(d *diagnostic.DebugContext, line int32, condition string) error {
	if err := d.NotifyAssertionViolation("src/util/heap.h", line, condition); err != nil {
		return err
	}
	return d.InvokeAssertionDebugger()
}
func (q *solvedQueue) debugInvariant() error {
	d := q.debugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	enabled, err = d.IsDebugEnabled("heap")
	if err != nil || !enabled {
		return err
	}
	for i, v := range q.values {
		pos, found := q.positions[v]
		if v < 0 || v >= q.bounds || !found || pos != i {
			if err := q.assertFailure(d, 63, "m_value2indices[m_values[idx]] == idx"); err != nil {
				return err
			}
		}
		if i > 0 && q.Less(i, (i-1)/2) {
			if err := q.assertFailure(d, 64, "parent(idx) == 0 || !less_than(m_values[idx], m_values[parent(idx)])"); err != nil {
				return err
			}
		}
	}
	return nil
}
func (q *solvedQueue) Empty() bool { return q == nil || len(q.values) == 0 }
func (q *solvedQueue) Bounds() int32 {
	if q == nil || q.closed {
		return 0
	}
	return q.bounds
}
func (q *solvedQueue) Contains(v int32) bool {
	if q == nil || q.closed || v < 0 || v >= q.bounds {
		return false
	}
	_, found := q.positions[v]
	return found
}
func (q *solvedQueue) CheckInvariant() bool {
	if q == nil || q.closed || len(q.positions) != len(q.values) {
		return false
	}
	for i, v := range q.values {
		pos, found := q.positions[v]
		if v < 0 || v >= q.bounds || !found || pos != i || i > 0 && q.Less(i, (i-1)/2) {
			return false
		}
	}
	return true
}
func (q *solvedQueue) Reserve(n int32) error {
	if err := q.alive(); err != nil {
		return err
	}
	if err := q.debugInvariant(); err != nil {
		return err
	}
	if n < 0 {
		return arena.ErrBounds
	}
	if n > q.bounds {
		q.bounds = n
	}
	return q.debugInvariant()
}
func (q *solvedQueue) Insert(v int32) error {
	if err := q.alive(); err != nil {
		return err
	}
	if err := q.debugInvariant(); err != nil {
		return err
	}
	if v < 0 || v >= q.bounds || q.Contains(v) {
		return errSolvedQueueValue
	}
	heap.Push(q, v)
	return q.debugInvariant()
}
func (q *solvedQueue) Erase(v int32) error {
	if err := q.alive(); err != nil {
		return err
	}
	if err := q.debugInvariant(); err != nil {
		return err
	}
	i, found := q.positions[v]
	if !found {
		return errSolvedQueueValue
	}
	heap.Remove(q, i)
	return q.debugInvariant()
}
func (q *solvedQueue) EraseMin() (int32, error) {
	if err := q.alive(); err != nil {
		return 0, err
	}
	if err := q.debugInvariant(); err != nil {
		return 0, err
	}
	if q.Empty() {
		return 0, errSolvedQueueValue
	}
	v := heap.Pop(q).(int32)
	return v, q.debugInvariant()
}
func (q *solvedQueue) Reset() error {
	if err := q.alive(); err != nil {
		return err
	}
	if err := q.debugInvariant(); err != nil {
		return err
	}
	clear(q.values)
	q.values = q.values[:0]
	clear(q.positions)
	return q.debugInvariant()
}
func (q *solvedQueue) Close() error {
	if q == nil || q.owner == nil {
		return arena.ErrInvalidSlot
	}
	if q.closed {
		return nil
	}
	q.values = nil
	q.positions = nil
	q.closed = true
	return nil
}
