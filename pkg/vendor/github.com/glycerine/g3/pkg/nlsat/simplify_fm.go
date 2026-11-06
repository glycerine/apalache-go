package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:480-638.

// SimplifyFourierMotzkin eliminates topology-ready variables whose every
// occurrence is a unit, odd, single-polynomial linear inequality with an
// invertible coefficient. Unsupported variables are left unchanged.
func (s *Solver) SimplifyFourierMotzkin() (count uint32, supported bool, err error) {
	if err = s.open(); err != nil {
		return 0, false, err
	}
	root, err := s.originalClausesHaveRootAtom()
	if err != nil || root {
		return 0, !root, err
	}
	variables := len(s.clauses.vars.isInt)
	for {
		var pass uint32
		for x := variables; x > 0; x-- {
			// Source compute_occurs adds newly published resolvents to lower-variable
			// occurrence lists. Rebuilding the concrete list here has the same order.
			collector := &subsumptionSimplifier{solver: s, hashes: map[*Clause]uint32{}, marked: map[*Clause]bool{}, removed: map[*Clause]bool{}}
			for _, clause := range s.clauses.originals {
				if err = collector.computeOccurrences(clause); err != nil {
					return count, false, err
				}
			}
			var occurrences []*Clause
			if x <= len(collector.occurrences) {
				occurrences = collector.occurrences[x-1]
			}
			changed, cause := s.applyFourierMotzkin(uint32(x-1), occurrences)
			if cause != nil {
				return count, false, cause
			}
			if changed {
				count++
				pass++
			}
		}
		if pass == 0 {
			break
		}
	}
	return count, true, nil
}
