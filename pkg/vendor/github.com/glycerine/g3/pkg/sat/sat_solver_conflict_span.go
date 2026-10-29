package sat

// Source: src/sat/sat_solver.cpp; src/sat/sat_clause.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	"sort"
)

// A borrowed literal span over caller Go storage, native literal backing, or
// a native unsigned column. Captured backing is never silently rebound.
type satSolverLiteralSpan struct {
	literals []Literal
	uints    []uint32
	column   *satUintColumn
	store    *satSolverLiteralStore
	backing  *satLiteralBacking
}

func satConflictColumnSpan(c *satUintColumn) satSolverLiteralSpan {
	return satSolverLiteralSpan{column: c, uints: c.values}
}

// Captures the actual c_ptr payload; later growth never silently rebinds it.
func satSimplifierCapturedSpan(s *satSolverLiteralStore, r satSolverLiteralListRef) (satSolverLiteralSpan, error) {
	rec, e := s.record(r)
	if e != nil {
		return satSolverLiteralSpan{}, e
	}
	return satSolverLiteralSpan{store: s, backing: rec.backing}, nil
}
func (s satSolverLiteralSpan) literal(i uint32) (Literal, error) {
	if s.column != nil {
		if e := s.column.open(); e != nil {
			return 0, e
		}
		if uint64(i) >= uint64(len(s.uints)) {
			return 0, arena.ErrBounds
		}
		return Literal(s.uints[i]), nil
	}
	if s.store != nil {
		if e := s.store.open(); e != nil {
			return 0, e
		}
		if s.backing == nil || !s.backing.alive {
			return 0, arena.ErrInvalidSlot
		}
		if uint64(i) >= uint64(len(s.backing.data)) {
			return 0, arena.ErrBounds
		}
		return s.backing.data[i], nil
	}
	if uint64(i) >= uint64(len(s.literals)) {
		return 0, arena.ErrBounds
	}
	return s.literals[i], nil
}

// Writes the actual captured pointer; it never rebinds to a relocated member.
func (s satSolverLiteralSpan) setLiteral(i uint32, l Literal) error {
	if s.column != nil {
		if e := s.column.open(); e != nil {
			return e
		}
		if uint64(i) >= uint64(len(s.uints)) {
			return arena.ErrBounds
		}
		s.uints[i] = uint32(l)
		return nil
	}
	if s.store != nil {
		if e := s.store.open(); e != nil {
			return e
		}
		if s.backing == nil || !s.backing.alive {
			return arena.ErrInvalidSlot
		}
		if uint64(i) >= uint64(len(s.backing.data)) {
			return arena.ErrBounds
		}
		s.backing.data[i] = l
		return nil
	}
	if uint64(i) >= uint64(len(s.literals)) {
		return arena.ErrBounds
	}
	s.literals[i] = l
	return nil
}

// Sort captured native values with Go's standard sorting implementation.
func satSortClauseSpan(s satSolverLiteralSpan, n uint32, sorter *satLiteralOrder) error {
	if s.column != nil {
		if e := s.column.open(); e != nil {
			return e
		}
		if uint64(n) > uint64(len(s.uints)) {
			return arena.ErrBounds
		}
		values := s.uints[:n]
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		return nil
	}
	if s.store != nil {
		if e := s.store.open(); e != nil {
			return e
		}
		if s.backing == nil || !s.backing.alive {
			return arena.ErrInvalidSlot
		}
		if uint64(n) > uint64(len(s.backing.data)) {
			return arena.ErrBounds
		}
		sorter.sort(s.backing.data[:n])
		return nil
	}
	if uint64(n) > uint64(len(s.literals)) {
		return arena.ErrBounds
	}
	sorter.sort(s.literals[:n])
	return nil
}
