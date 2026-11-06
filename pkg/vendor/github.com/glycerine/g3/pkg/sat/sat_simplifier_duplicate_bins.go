package sat

// Source: src/sat/sat_simplifier.cpp:736–784.

import "sort"

func satSimplifierBinaryLess(a, b satWatched) bool {
	if !a.Binary() {
		return false
	}
	if !b.Binary() {
		return true
	}
	x, y := a.val1, b.val1
	if x < y {
		return true
	}
	if x == y {
		return !a.LearnedUnchecked() && b.LearnedUnchecked()
	}
	return false
}
func (s *satSimplifier) sortBinaryWatches(w satSolverWatchListView) error {
	it, e := s.watchIterator(w)
	if e != nil {
		return e
	}
	if it.end < 2 {
		return nil
	}
	data := it.backing.data[:it.end]
	sort.SliceStable(data, func(i, j int) bool { return satSimplifierBinaryLess(data[i], data[j]) })
	return nil
}
func (s *satSimplifier) eliminateDuplicateBinaries() error {
	p := s.lifecycle.propagation
	ws := p.variables.watches
	elim := uint32(0)
	end := uint32(len(ws.lists))
	for idx := uint32(0); idx < end; idx++ {
		if e := p.checkpoint(); e != nil {
			return e
		}
		w, e := ws.Get(idx)
		if e != nil {
			return e
		}
		if e = s.sortBinaryWatches(w); e != nil {
			return e
		}
		it, e := s.watchIterator(w)
		if e != nil {
			return e
		}
		last := DefaultLiteral()
		out := uint32(0)
		for i := uint32(0); i < it.end; i++ {
			x, e := satPropagationCursorRead(&it, i)
			if e != nil {
				return e
			}
			if !x.Binary() {
				if e = satPropagationCursorWrite(&it, out, x); e != nil {
					return e
				}
				out++
				continue
			}
			l, e := x.Literal()
			if e != nil {
				return e
			}
			if l == last {
				s.traceDuplicateBinary(idx, l)
				elim++
			} else {
				last, e = x.Literal()
				if e != nil {
					return e
				}
				if e = satPropagationCursorWrite(&it, out, x); e != nil {
					return e
				}
				out++
			}
		}
		if e = p.finishPropagationList(&it, w, out); e != nil {
			return e
		}
	}
	s.numSubsumed += elim / 2
	return nil
}
