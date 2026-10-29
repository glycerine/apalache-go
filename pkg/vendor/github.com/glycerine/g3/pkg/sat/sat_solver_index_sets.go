package sat

// Concrete SAT antecedent storage. Container backing uses native Go collections.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/insdict"
)

type satSolverIndexSet struct {
	keys   insdict.Dict[uint32, struct{}]
	view   *satSolverIndexView
	closed bool
}
type satSolverIndexMap struct {
	self       *satSolverIndexMap
	budget     *arena.Budget
	entries    map[uint32]*satSolverIndexSet
	ownerBytes uint64
	closed     bool
}

func newSATSolverIndexMap(b *arena.Budget) (*satSolverIndexMap, error) {
	bytes := uint64(reflect.TypeOf(satSolverIndexMap{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	m := &satSolverIndexMap{budget: b, ownerBytes: bytes, entries: make(map[uint32]*satSolverIndexSet)}
	m.self = m
	return m, nil
}
func (m *satSolverIndexMap) Close() error {
	if m == nil || m.self != m {
		return ErrSATOwner
	}
	if m.closed {
		return nil
	}
	for _, row := range m.entries {
		row.close()
	}
	m.entries = nil
	m.budget.Release(m.ownerBytes)
	m.ownerBytes = 0
	m.closed = true
	return nil
}
