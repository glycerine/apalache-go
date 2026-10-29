package sat

// Source: src/sat/sat_solver.h:517-528; src/sat/sat_solver.cpp:2804-2812.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Actual solver m_activity uses native uint32 scores. Queue comparisons borrow
// this owner and always read its current slice.
type satSolverActivity struct {
	self       *satSolverActivity
	values     []uint32
	budget     *arena.Budget
	ownerBytes uint64
	closed     bool
}

func newSATSolverActivity(b *arena.Budget) (*satSolverActivity, error) {
	if b == nil {
		return nil, arena.ErrConfig
	}
	n := uint64(reflect.TypeOf(satSolverActivity{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	a := &satSolverActivity{budget: b, ownerBytes: n}
	a.self = a
	return a, nil
}
func (a *satSolverActivity) open() error {
	if a == nil || a.self != a {
		return ErrSATOwner
	}
	if a.closed {
		return arena.ErrClosed
	}
	return nil
}
func (a *satSolverActivity) Get(i uint32) (uint32, error) {
	if e := a.open(); e != nil {
		return 0, e
	}
	if uint64(i) >= uint64(len(a.values)) {
		if satStorageSourceDebug {
			return 0, ErrSATAssertion
		}
		return 0, arena.ErrBounds
	}
	return a.values[i], nil
}
func (a *satSolverActivity) Set(i, x uint32) error {
	if _, e := a.Get(i); e != nil {
		return e
	}
	a.values[i] = x
	return nil
}
func (a *satSolverActivity) Push(x uint32) error {
	if e := a.open(); e != nil {
		return e
	}
	a.values = append(a.values, x)
	return nil
}
func (a *satSolverActivity) Size() uint32 {
	if a.open() != nil {
		return 0
	}
	return uint32(len(a.values))
}
func (a *satSolverActivity) Stats() satStorageVectorStats {
	if a.open() != nil {
		return satStorageVectorStats{}
	}
	return satStorageVectorStats{OwnerBytes: a.ownerBytes}
}
func (a *satSolverActivity) Close() error {
	if a == nil || a.self != a {
		return ErrSATOwner
	}
	if a.closed {
		return nil
	}
	a.values = nil
	a.budget.Release(a.ownerBytes)
	a.ownerBytes = 0
	a.closed = true
	return nil
}

// sat_solver.h:517-528 and sat_solver.cpp:2804-2812. Queue event precedes
// rescaling; neither multiplication nor addition is widened beyond uint32.
func (v *satSolverVariableState) incActivity(id BoolVar) error {
	if e := v.open(); e != nil {
		return e
	}
	act, e := v.activity.Get(uint32(id))
	if e != nil {
		return e
	}
	if e = v.activity.Set(uint32(id), act+v.activityInc); e != nil {
		return e
	}
	if e = v.queue.ActivityIncreased(id); e != nil {
		return e
	}
	act, e = v.activity.Get(uint32(id))
	if e != nil {
		return e
	}
	if act > 1<<24 {
		return v.rescaleActivity()
	}
	return nil
}
func (v *satSolverVariableState) decayActivity() { v.activityInc *= 11; v.activityInc /= 10 }
func (v *satSolverVariableState) rescaleActivity() error {
	if e := v.open(); e != nil {
		return e
	}
	if e := v.activity.open(); e != nil {
		return e
	}
	for i := range v.activity.values {
		v.activity.values[i] >>= 14
	}
	v.activityInc >>= 14
	return nil
}
