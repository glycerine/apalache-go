package sat

// Source: src/sat/sat_simplifier.cpp; src/sat/sat_simplifier.h.
// Concrete extracted state for sat::simplifier's solver-independent algorithms.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// This is the actual visited plane and two source counters, not a solver or a
// simplifier Run implementation. The eventual solver-bound simplifier owns this
// subobject. Clause/use-list lifetimes remain with their actual callers.
type satSimplifierKernel struct {
	self                    *satSimplifierKernel
	allocator               *ClauseAllocator
	lifecycle               *satSolverClauseLifecycle
	dummy                   *ClauseAllocator
	useList                 *satClauseUseLists
	budget                  *arena.Budget
	visited                 []uint8
	length                  uint32
	subCounter, elimCounter int32
	ownerBytes              uint64
	closed                  bool
}
type satSimplifierKernelStats struct {
	OwnerBytes     uint64
	Size, Capacity uint32
}

func newSATSimplifierKernel(a *ClauseAllocator, u *satClauseUseLists, b *arena.Budget) (*satSimplifierKernel, error) {
	if e := a.open(); e != nil {
		return nil, e
	}
	if e := u.open(); e != nil {
		return nil, e
	}
	if u.allocator != a {
		return nil, ErrSATOwner
	}
	n := uint64(reflect.TypeOf(satSimplifierKernel{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	k := &satSimplifierKernel{allocator: a, useList: u, budget: b, ownerBytes: n}
	k.self = k
	return k, nil
}
func (k *satSimplifierKernel) open() error {
	if k == nil || k.self != k {
		return ErrSATOwner
	}
	if k.closed {
		return arena.ErrClosed
	}
	return nil
}
func (k *satSimplifierKernel) Stats() satSimplifierKernelStats {
	if k == nil || k.closed {
		return satSimplifierKernelStats{}
	}
	return satSimplifierKernelStats{k.ownerBytes, k.length, uint32(len(k.visited))}
}

func (k *satSimplifierKernel) InitVisited(numVars uint32) error {
	if e := k.open(); e != nil {
		return e
	}
	k.length = 0 // source reset BEFORE unsigned multiply/reserve.
	n := 2 * numVars
	if uint64(n) > uint64(len(k.visited)) {
		k.visited = append(k.visited, make([]uint8, int(n)-len(k.visited))...)
	}
	k.length = n
	clear(k.visited[:n])
	return nil
}
func (k *satSimplifierKernel) visitedIndex(l Literal) (uint32, error) {
	if e := k.open(); e != nil {
		return 0, e
	}
	i := l.index()
	if satStorageSourceDebug && i >= k.length {
		return 0, ErrSATAssertion
	}
	if uint64(i) >= uint64(len(k.visited)) {
		return 0, arena.ErrBounds
	}
	return i, nil
}
func (k *satSimplifierKernel) markVisited(l Literal) error {
	i, e := k.visitedIndex(l)
	if e == nil {
		k.visited[i] = 1
	}
	return e
}
func (k *satSimplifierKernel) unmarkVisited(l Literal) error {
	i, e := k.visitedIndex(l)
	if e == nil {
		k.visited[i] = 0
	}
	return e
}
func (k *satSimplifierKernel) isMarked(l Literal) (bool, error) {
	i, e := k.visitedIndex(l)
	if e != nil {
		return false, e
	}
	return k.visited[i] != 0, nil
}
func (k *satSimplifierKernel) markAllBut(c StoredClauseRef, except Literal) error {
	if e := k.open(); e != nil {
		return e
	}
	n, e := k.clauseSize(c)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		l, e := k.clauseLiteral(c, i)
		if e != nil {
			return e
		}
		if l == except {
			continue
		}
		if e = k.markVisited(l); e != nil {
			return e
		}
	}
	return nil
}
func (k *satSimplifierKernel) unmarkAll(c StoredClauseRef) error {
	if e := k.open(); e != nil {
		return e
	}
	n, e := k.clauseSize(c)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		l, e := k.clauseLiteral(c, i)
		if e != nil {
			return e
		}
		if e = k.unmarkVisited(l); e != nil {
			return e
		}
	}
	return nil
}
func (k *satSimplifierKernel) Finalize() error {
	if e := k.open(); e != nil {
		return e
	}
	k.visited = nil
	k.length = 0
	return nil
}
func (k *satSimplifierKernel) Close() error {
	if k == nil || k.self != k {
		return ErrSATOwner
	}
	if k.closed {
		return nil
	}
	if e := k.Finalize(); e != nil {
		return e
	}
	k.budget.Release(k.ownerBytes)
	k.ownerBytes = 0
	k.closed = true
	return nil
}
