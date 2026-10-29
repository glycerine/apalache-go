package sat

// Native antecedent map with independently copied ordered index sets.

import "github.com/glycerine/g3/pkg/arena"

func (m *satSolverIndexMap) open() error {
	if m == nil || m.self != m {
		return ErrSATOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	return nil
}
func (m *satSolverIndexMap) contains(k uint32) (bool, error) {
	if e := m.open(); e != nil {
		return false, e
	}
	_, yes := m.entries[k]
	return yes, nil
}
func (m *satSolverIndexMap) find(k uint32) (*satSolverIndexSet, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	if row, yes := m.entries[k]; yes {
		return row, nil
	}
	if satStorageSourceDebug {
		return nil, ErrSATAssertion
	}
	return nil, arena.ErrBounds
}
func (m *satSolverIndexMap) insert(k uint32, value *satSolverIndexSet) error {
	if e := m.open(); e != nil {
		return e
	}
	if e := value.open(); e != nil {
		return e
	}
	row := m.entries[k]
	if row == nil {
		row = &satSolverIndexSet{}
		m.entries[k] = row
	}
	return row.assign(value)
}
func (m *satSolverIndexMap) reset() error {
	if e := m.open(); e != nil {
		return e
	}
	for _, row := range m.entries {
		row.close()
	}
	clear(m.entries)
	return nil
}
