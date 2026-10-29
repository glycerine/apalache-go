package sat

// Source: src/sat/sat_asymm_branch.cpp:81–82,92–119.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Actual charged stable workspace is a Go physical ownership adapter, not a
// source malloc event. Disjoint moved destination/unconsumed source ranges own
// clauses; scalar duplicates are borrows and never a backup registry.
func (g *satAsymmBranch) less(a, b StoredClauseRef) (bool, error) {
	x := g.lifecycle.propagation.clauses
	n, e := x.Size(a)
	if e != nil {
		return false, e
	}
	m, e := x.Size(b)
	return n > m, e
}
func (g *satAsymmBranch) sortSource() []StoredClauseRef {
	if g.sortSrcScratch {
		return g.scratch
	}
	return g.lifecycle.Original
}
func (g *satAsymmBranch) sortDestination() []StoredClauseRef {
	if g.sortSrcScratch {
		return g.lifecycle.Original
	}
	return g.scratch
}
func (g *satAsymmBranch) sortOriginal() error {
	l := g.lifecycle
	n := uint32(len(l.Original))
	if n < 2 {
		return nil
	}
	if l.cleanerContinuation != nil || l.gcContinuation != nil || l.asymmContinuation != nil {
		return ErrSATOwner
	}
	bytes := uint64(n) * uint64(reflect.TypeOf(StoredClauseRef{}).Size())
	if bytes > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if e := g.lifecycle.budget.Reserve(bytes); e != nil {
		return e
	}
	g.scratch = make([]StoredClauseRef, n)
	g.scratchBytes = bytes
	g.sortN = n
	g.sortSrcScratch = false
	g.sorting = true
	g.running = true
	l.asymmContinuation = g
	defer func() { g.running = false }()
	for width := uint64(1); width < uint64(n); width *= 2 {
		g.sortOut = 0
		for base := uint64(0); base < uint64(n); base += 2 * width {
			mid := base + width
			if mid > uint64(n) {
				mid = uint64(n)
			}
			end := base + 2*width
			if end > uint64(n) {
				end = uint64(n)
			}
			g.sortLeft = uint32(base)
			g.sortMid = uint32(mid)
			g.sortRight = uint32(mid)
			g.sortEnd = uint32(end)
			for g.sortLeft < g.sortMid || g.sortRight < g.sortEnd {
				chooseRight := false
				if g.sortLeft == g.sortMid {
					chooseRight = true
				} else if g.sortRight < g.sortEnd {
					yes, e := g.less(g.sortSource()[g.sortRight], g.sortSource()[g.sortLeft])
					if e != nil {
						return e
					}
					chooseRight = yes
				}
				idx := g.sortLeft
				if chooseRight {
					idx = g.sortRight
				}
				g.sortDestination()[g.sortOut] = g.sortSource()[idx]
				g.sortOut++
				if chooseRight {
					g.sortRight++
				} else {
					g.sortLeft++
				}
			}
		}
		g.sortSrcScratch = !g.sortSrcScratch
	}
	if g.sortSrcScratch {
		copy(l.Original, g.scratch)
	}
	g.sorting = false
	l.asymmContinuation = nil
	g.lifecycle.budget.Release(g.scratchBytes)
	g.scratchBytes = 0
	g.scratch = nil
	return nil
}
func (g *satAsymmBranch) cleanupSort() error {
	if !g.sorting {
		return nil
	}
	l := g.lifecycle
	a := l.propagation.clauses
	for g.sortDrainStage < 4 {
		var data []StoredClauseRef
		lo, hi := uint32(0), uint32(0)
		switch g.sortDrainStage {
		case 0:
			data = g.sortDestination()
			hi = g.sortOut
		case 1:
			data = g.sortSource()
			lo = g.sortLeft
			hi = g.sortMid
		case 2:
			data = g.sortSource()
			lo = g.sortRight
			hi = g.sortEnd
		case 3:
			data = g.sortSource()
			lo = g.sortEnd
			hi = g.sortN
		}
		if g.sortDrainIndex < lo {
			g.sortDrainIndex = lo
		}
		for g.sortDrainIndex < hi {
			if uint64(g.sortDrainIndex) >= uint64(len(data)) {
				return arena.ErrBounds
			}
			if e := a.Delete(data[g.sortDrainIndex]); e != nil {
				return e
			}
			g.sortDrainIndex++
		}
		g.sortDrainStage++
		g.sortDrainIndex = 0
	}
	l.Original = l.Original[:0]
	g.sorting = false
	l.asymmContinuation = nil
	g.lifecycle.budget.Release(g.scratchBytes)
	g.scratchBytes = 0
	g.scratch = nil
	return nil
}
func (g *satAsymmBranch) cleanupInterrupted() error {
	if g.sorting {
		return g.cleanupSort()
	}
	if !g.interrupted {
		return nil
	}
	l := g.lifecycle
	if l.asymmContinuation != g {
		return ErrSATOwner
	}
	if uint32(len(l.Original)) < g.end {
		return arena.ErrBounds
	}
	a := l.propagation.clauses
	for g.drainPrefix < g.out {
		if e := a.Delete(l.Original[g.drainPrefix]); e != nil {
			return e
		}
		g.drainPrefix++
	}
	if g.drainSuffix < g.cursor {
		g.drainSuffix = g.cursor
	}
	for g.drainSuffix < g.end {
		if e := a.Delete(l.Original[g.drainSuffix]); e != nil {
			return e
		}
		g.drainSuffix++
	}
	l.Original = l.Original[:0]
	g.interrupted = false
	g.detachActive = false
	g.detachedClause = StoredClauseRef{}
	l.asymmContinuation = nil
	return nil
}
