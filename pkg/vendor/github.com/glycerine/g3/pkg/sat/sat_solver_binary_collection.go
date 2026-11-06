package sat

// Source: src/sat/sat_solver.cpp:2818-2838.

// appendBinaryClauses is the single retained implementation of
// solver::collect_bin_clauses. The destination is caller-owned and its existing
// prefix is preserved. When learned is false, learned binary watches are
// filtered; when it is true, both original and learned watches are appended.
//
// The source stores each binary clause in two watch lists. The literal-index
// comparison below selects the one canonical visit and, importantly, the
// cleanup_bug trace is emitted after that selection and before publication.
func (s *Solver) appendBinaryClauses(out []BinaryLiteralPair, learned bool) ([]BinaryLiteralPair, error) {
	for idx := 0; idx < len(s.Variables.watches.lists); idx++ {
		l := Literal(uint32(idx)).Negated()
		list, err := s.Variables.watches.Get(uint32(idx))
		if err != nil {
			return out, err
		}
		ref, err := list.ref()
		if err != nil {
			return out, err
		}
		it, err := s.Variables.watches.store.Iter(ref)
		if err != nil {
			return out, err
		}
		for {
			watch, yes, err := it.Next()
			if err != nil {
				return out, err
			}
			if !yes {
				break
			}
			if !watch.Binary() {
				continue
			}
			if !learned {
				isLearned, err := watch.Learned()
				if err != nil {
					return out, err
				}
				if isLearned {
					continue
				}
			}
			other, err := watch.Literal()
			if err != nil {
				return out, err
			}
			if l.index() > other.index() {
				continue
			}
			s.traceCollectedBinClause(l, other)
			out = append(out, newSATSolverBinPair(l, other))
		}
	}
	return out, nil
}
