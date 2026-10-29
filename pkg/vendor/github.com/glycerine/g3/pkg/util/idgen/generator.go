package idgen

// Source: src/util/id_gen.h:25-85; src/util/vector.h.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

// Generator assigns monotonically increasing IDs and reuses released IDs in
// LIFO order. The free list is owned by the generator; copying a generator
// after first use aliases that storage.
type Generator struct {
	next        uint32
	free        []uint32
	outOfMemory func() bool
}

func New(start uint32, outOfMemory func() bool) Generator {
	return Generator{next: start, outOfMemory: outOfMemory}
}

// Reserve ensures that at least needed released IDs can be retained.
func (g *Generator) Reserve(needed uint32) error {
	if uint64(needed) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if int(needed) > cap(g.free) {
		next := make([]uint32, len(g.free), int(needed))
		copy(next, g.free)
		g.free = next
	}
	return nil
}

// Mk returns the most recently recycled ID, or a fresh ID when none is free.
func (g *Generator) Mk() uint32 {
	if len(g.free) == 0 {
		id := g.next
		g.next++
		return id
	}
	last := len(g.free) - 1
	id := g.free[last]
	g.free = g.free[:last]
	return id
}

func (g *Generator) Recycle(id uint32) error {
	if g.outOfMemory != nil && g.outOfMemory() {
		return nil
	}
	if uint64(len(g.free)) >= math.MaxUint32 {
		return arena.ErrBounds
	}
	g.free = append(g.free, id)
	return nil
}

func (g *Generator) Reset(start uint32) {
	g.next = start
	g.free = g.free[:0]
}

func (g *Generator) Cleanup(start uint32) {
	g.next = start
	g.free = nil
}

func (g *Generator) Range() uint32     { return g.next }
func (g *Generator) FreeCapacity() int { return cap(g.free) }
func (g *Generator) FreeLen() int      { return len(g.free) }
func (g *Generator) IsOutOfMemory() bool {
	return g.outOfMemory != nil && g.outOfMemory()
}

// FreeIDs returns a borrowed, read-only view for owner diagnostics.
func (g *Generator) FreeIDs() []uint32 { return g.free }

// SetNextID supports the source repro/debug path. It finds the first ID at or
// above id that is absent from the free list without consuming that list.
func (g *Generator) SetNextID(id uint32) uint32 {
	g.next = id
	for {
		found := false
		for _, free := range g.free {
			if free == g.next {
				found = true
				break
			}
		}
		if !found {
			return g.next
		}
		g.next++
	}
}
