package memory

// Source: src/util/small_object_allocator.cpp:172–242; src/util/debug.h:63.

import (
	"math"
	"sort"
)

func (a *SmallAllocator) Consolidate() error {
	if e := a.open(); e != nil {
		return e
	}
	if a.hasPending() || a.cleanupMode != 0 || a.cleanupChunksNeeded {
		return ErrAllocatorPending
	}
	if e := a.verbose(false); e != nil {
		return e
	}
	var chunks, freeObjs []uint32
	for slot := uint32(1); slot < a.slots(); slot++ {
		if a.free[slot] == 0 {
			continue
		}
		chunks = chunks[:0]
		freeObjs = freeObjs[:0]
		id := a.chunks[slot]
		steps := 0
		for id != 0 {
			if steps >= len(a.locations) {
				return ErrAllocatorNonterminating
			}
			steps++
			chunks = append(chunks, id)
			c, e := a.location(id)
			if e != nil {
				return e
			}
			next, e := smallObjectReadWord(a.config.Runtime, c.raw, 0, a.pointerBytes())
			if e != nil {
				return e
			}
			if next > math.MaxUint32 {
				return ErrAllocatorOwner
			}
			id = uint32(next)
		}
		id = a.free[slot]
		steps = 0
		for id != 0 {
			if steps >= len(a.locations) {
				return ErrAllocatorNonterminating
			}
			steps++
			freeObjs = append(freeObjs, id)
			obj, e := a.location(id)
			if e != nil {
				return e
			}
			next, e := smallObjectReadWord(a.config.Runtime, obj.raw, uint64(obj.offset), a.pointerBytes())
			if e != nil {
				return e
			}
			if next > math.MaxUint32 {
				return ErrAllocatorOwner
			}
			id = uint32(next)
		}
		objectSize := uint32(a.pointerBytes()) * slot
		perChunk := (8192 - 2*uint32(a.pointerBytes())) / objectSize
		if uint32(len(freeObjs)) < perChunk {
			continue
		}
		if e := a.sourceAssert(len(chunks) != 0); e != nil {
			return e
		}
		// Gathering validated every ID. Sort by the same scalar addresses used
		// by the source consolidation algorithm.
		sort.Slice(chunks, func(i, j int) bool {
			return a.locations[chunks[i]-1].address < a.locations[chunks[j]-1].address
		})
		sort.Slice(freeObjs, func(i, j int) bool {
			return a.locations[freeObjs[i]-1].address < a.locations[freeObjs[j]-1].address
		})
		var lastChunk, lastFree, objectIndex uint32
		for chunkIndex := uint32(0); chunkIndex < uint32(len(chunks)); chunkIndex++ {
			chunkID := chunks[chunkIndex]
			chunk, e := a.location(chunkID)
			if e != nil {
				return e
			}
			end := chunk.address + 8192
			saved := objectIndex
			var count uint32
			for objectIndex < uint32(len(freeObjs)) {
				objectID := freeObjs[objectIndex]
				obj, e := a.location(objectID)
				if e != nil {
					return e
				}
				if obj.address > end {
					break
				}
				objectIndex++
				count++
			}
			if count == perChunk {
				h := chunk.raw
				a.cleanupChunksNeeded = true
				e = a.config.Runtime.Deallocate(&h)
				if h == (Block{}) {
					if a.config.Profile.Debug && a.config.ChunkTrace != nil {
						a.config.ChunkTrace.DeallocatedChunk(chunk.address, 221)
					}
					a.invalidateChunk(chunkID)
				}
				if e != nil {
					return e
				}
			} else {
				if e := smallObjectWriteWord(a.config.Runtime, chunk.raw, 0, a.pointerBytes(), uint64(lastChunk)); e != nil {
					return e
				}
				lastChunk = chunkID
				for i := saved; i < objectIndex; i++ {
					objectID := freeObjs[i]
					obj, e := a.location(objectID)
					if e != nil {
						return e
					}
					if e = smallObjectWriteWord(a.config.Runtime, obj.raw, uint64(obj.offset), a.pointerBytes(), uint64(lastFree)); e != nil {
						return e
					}
					lastFree = objectID
				}
			}
		}
		a.chunks[slot] = lastChunk
		a.free[slot] = lastFree
	}
	a.cleanupChunksNeeded = false
	return a.verbose(true)
}
