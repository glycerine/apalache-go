package sat

// Source: src/sat/sat_probing.cpp; src/sat/sat_probing.h.

import "github.com/glycerine/g3/pkg/arena"

// Native cache rows retain the source availability flag and concrete literals.
type satProbingCacheRow struct {
	available bool
	values    []uint32
}
type satProbingCache struct {
	rows  []satProbingCacheRow
	epoch uint64
}

func (c *satProbingCache) blockLive() bool                          { return len(c.rows) != 0 }
func (c *satProbingCache) size(_ *satSolverProbing) (uint32, error) { return uint32(len(c.rows)), nil }
func (c *satProbingCache) reserve(_ *satSolverProbing, needed uint32) error {
	if uint64(needed) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if int(needed) <= len(c.rows) {
		return nil
	}
	if int(needed) > cap(c.rows) {
		if c.epoch == ^uint64(0) {
			return arena.ErrBounds
		}
		c.epoch++
	}
	c.rows = append(c.rows, make([]satProbingCacheRow, int(needed)-len(c.rows))...)
	return nil
}
func (c *satProbingCache) workRow(p *satSolverProbing, i uint32) (*satUintColumn, error) {
	if uint64(i) >= uint64(len(c.rows)) {
		return nil, arena.ErrBounds
	}
	out := &p.cacheWork
	out.values = c.rows[i].values
	out.closed = false
	c.rows[i].values = nil
	return out, nil
}
func (c *satProbingCache) saveWork(p *satSolverProbing, i uint32) {
	out := &p.cacheWork
	c.rows[i].values = out.values
	out.values = nil
	out.closed = false
}
func (c *satProbingCache) finalize(_ *satSolverProbing) error { c.rows = nil; return nil }

// A row borrow checks the current outer storage epoch. A captured values slice
// retains its actual backing and is never rebound to a later cache row.
type satProbingCacheBorrow struct {
	owner *satSolverProbing
	outer uint64
	index uint32
}

func (b satProbingCacheBorrow) row() (satProbingCacheRow, error) {
	if b.owner == nil {
		return satProbingCacheRow{}, ErrSATOwner
	}
	if e := b.owner.open(); e != nil {
		return satProbingCacheRow{}, e
	}
	if b.outer != b.owner.cache.epoch {
		return satProbingCacheRow{}, ErrSATOwner
	}
	if uint64(b.index) >= uint64(len(b.owner.cache.rows)) {
		return satProbingCacheRow{}, arena.ErrBounds
	}
	return b.owner.cache.rows[b.index], nil
}
func (b satProbingCacheBorrow) size() (uint32, error) {
	row, e := b.row()
	if e != nil {
		return 0, e
	}
	return uint32(len(row.values)), nil
}
func (b satProbingCacheBorrow) capture() ([]uint32, uint32, error) {
	row, e := b.row()
	if e != nil {
		return nil, 0, e
	}
	return row.values, uint32(len(row.values)), nil
}
func (b satProbingCacheBorrow) readCaptured(values []uint32, i uint32) (Literal, error) {
	if b.owner == nil {
		return 0, ErrSATOwner
	}
	if e := b.owner.open(); e != nil {
		return 0, e
	}
	if uint64(i) >= uint64(len(values)) {
		return 0, arena.ErrBounds
	}
	return Literal(values[i]), nil
}
