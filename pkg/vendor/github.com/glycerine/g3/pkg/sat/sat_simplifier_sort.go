package sat

// Source: src/sat/sat_simplifier.cpp:83–96; std::stable_sort invocation.

import "sort"

func (s *satSimplifier) clauseVector(learned bool) []StoredClauseRef {
	if learned {
		return s.lifecycle.learned
	}
	return s.lifecycle.Original
}

// Clause sizes are stable through this sort. Capture them before sorting so the
// standard-library comparator cannot fail or change the owning clause vector
// when a Go checked handle reports an invalid source-domain input.
func (s *satSimplifier) sortClauses(learned bool) error {
	cs := s.clauseVector(learned)
	if len(cs) < 2 {
		return nil
	}
	type sizedClause struct {
		ref  StoredClauseRef
		size uint32
	}
	entries := make([]sizedClause, len(cs))
	for i, ref := range cs {
		size, e := s.lifecycle.propagation.clauses.Size(ref)
		if e != nil {
			return e
		}
		entries[i] = sizedClause{ref: ref, size: size}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].size > entries[j].size })
	for i, entry := range entries {
		cs[i] = entry.ref
	}
	return nil
}
func (s *satSimplifier) registerClauses(learned bool) error {
	if e := s.sortClauses(learned); e != nil {
		return e
	}
	cs := s.clauseVector(learned)
	n := uint32(len(cs))
	for i := uint32(0); i < n; i++ {
		c := cs[i]
		f, e := s.lifecycle.propagation.clauses.Frozen(c)
		if e != nil {
			return e
		}
		if !f {
			if e = s.useList.Insert(c); e != nil {
				return e
			}
			b, e := s.lifecycle.propagation.clauses.Strengthened(c)
			if e != nil {
				return e
			}
			if b {
				if e = s.subTodo.Insert(c); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
