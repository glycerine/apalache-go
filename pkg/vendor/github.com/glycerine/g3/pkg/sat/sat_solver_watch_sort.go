package sat

import "sort"

type satWatchSortSlice struct{ data []satWatched }

func (v *satWatchSortSlice) Len() int           { return len(v.data) }
func (v *satWatchSortSlice) Less(i, j int) bool { return satWatchedLess(v.data[i], v.data[j]) }
func (v *satWatchSortSlice) Swap(i, j int)      { v.data[i], v.data[j] = v.data[j], v.data[i] }

// watched_lt orders three source watch classes and treats entries within a
// class as equivalent. Go's stable sort keeps their original insertion order.
func (s *satSolverWatchStore) SortByKind(r satSolverWatchListRef) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	// The store owns the interface adapter, so sorting each watch list does
	// not allocate a fresh boxed slice during the solver's outer loop.
	s.sorter.data = x.values()
	sort.Stable(&s.sorter)
	s.sorter.data = nil
	return nil
}
