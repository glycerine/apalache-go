package sat

// Source: src/sat/sat_solver.cpp:1412–1469.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Concrete std::stable_sort boundary: stable merge workspace is real charged Go
// storage, not a source malloc event or Runtime ticket. Source pointers have LP64
// width8; this actual scalar-handle backing uses its reflected Go stride. During
// each move, cursors publish disjoint ownership in consumed destination prefix
// and unconsumed source ranges. Old copied scalar slots are borrowers only.
func (g *satSolverGC) less(a, b StoredClauseRef, kind satGCStrategy) (bool, error) {
	x := g.lifecycle().propagation.clauses
	first, second := 0, 0
	if kind == satGCGlue || kind == satGCGluePSM {
		first = 1
	} else {
		first = 2
	}
	if kind == satGCGluePSM {
		second = 2
	}
	if kind == satGCPSMGlue {
		second = 1
	}
	av, e := g.sortMeasure(a, first)
	if e != nil {
		return false, e
	}
	bv, e := g.sortMeasure(b, first)
	if e != nil {
		return false, e
	}
	if av < bv {
		return true, nil
	}
	av, e = g.sortMeasure(a, first)
	if e != nil {
		return false, e
	}
	bv, e = g.sortMeasure(b, first)
	if e != nil {
		return false, e
	}
	if second != 0 {
		if av > bv {
			return false, nil
		}
		av, e = g.sortMeasure(a, second)
		if e != nil {
			return false, e
		}
		bv, e = g.sortMeasure(b, second)
		if e != nil {
			return false, e
		}
		if av < bv {
			return true, nil
		}
		av, e = g.sortMeasure(a, second)
		if e != nil {
			return false, e
		}
		bv, e = g.sortMeasure(b, second)
		if e != nil {
			return false, e
		}
		if av > bv {
			return false, nil
		}
	} else if av != bv {
		return false, nil
	}
	n, e := x.Size(a)
	if e != nil {
		return false, e
	}
	m, e := x.Size(b)
	return n < m, e
}
func (g *satSolverGC) sortMeasure(r StoredClauseRef, k int) (uint32, error) {
	a := g.lifecycle().propagation.clauses
	if k == 1 {
		return a.Glue(r)
	}
	return a.PSM(r)
}
func (g *satSolverGC) sortSource() []StoredClauseRef {
	if g.sortSrcScratch {
		return g.scratch
	}
	return g.lifecycle().learned
}
func (g *satSolverGC) sortDestination() []StoredClauseRef {
	if g.sortSrcScratch {
		return g.lifecycle().learned
	}
	return g.scratch
}
func (g *satSolverGC) sortLearned(kind satGCStrategy) error {
	l := g.lifecycle()
	n := uint32(len(l.learned))
	if n < 2 {
		return nil
	}
	if l.cleanerContinuation != nil || l.gcContinuation != nil {
		return ErrSATOwner
	}
	bytes := uint64(n) * uint64(reflect.TypeOf(StoredClauseRef{}).Size())
	if bytes > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if e := g.budget().Reserve(bytes); e != nil {
		return e
	}
	g.scratch = make([]StoredClauseRef, n)
	g.scratchBytes = bytes
	g.sortN = n
	g.sortSrcScratch = false
	g.sorting = true
	g.running = true
	l.gcContinuation = g
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
					yes, e := g.less(g.sortSource()[g.sortRight], g.sortSource()[g.sortLeft], kind)
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
		copy(l.learned, g.scratch)
	}
	g.sorting = false
	l.gcContinuation = nil
	g.budget().Release(g.scratchBytes)
	g.scratchBytes = 0
	g.scratch = nil
	return nil
}
func (g *satSolverGC) cleanupSort() error {
	if !g.sorting {
		return nil
	}
	l := g.lifecycle()
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
	l.learned = l.learned[:0]
	g.sorting = false
	l.gcContinuation = nil
	g.budget().Release(g.scratchBytes)
	g.scratchBytes = 0
	g.scratch = nil
	return nil
}
func (g *satSolverGC) cleanupInterrupted() error {
	if g.sorting {
		return g.cleanupSort()
	}
	if !g.interrupted {
		return nil
	}
	l := g.lifecycle()
	if l.gcContinuation != g {
		return ErrSATOwner
	}
	if uint32(len(l.learned)) < g.end {
		return arena.ErrBounds
	}
	a := l.propagation.clauses
	for g.drainPrefix < g.out {
		if e := a.Delete(l.learned[g.drainPrefix]); e != nil {
			return e
		}
		g.drainPrefix++
	}
	if g.drainSuffix < g.cursor {
		g.drainSuffix = g.cursor
	}
	for g.drainSuffix < g.end {
		if e := a.Delete(l.learned[g.drainSuffix]); e != nil {
			return e
		}
		g.drainSuffix++
	}
	l.learned = l.learned[:0]
	g.interrupted = false
	l.gcContinuation = nil
	return nil
}
