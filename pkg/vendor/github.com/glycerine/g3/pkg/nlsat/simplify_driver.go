package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:20-57,837-849.

// SimplificationSummary reports completed source phases without changing the
// fixed-point decisions, which remain based solely on original clause count.
type SimplificationSummary struct {
	Rounds, LearnedRemoved, Subsumed, Unconstrained, ContentRewritten, FourierMotzkin, FactorsSplit uint32
}

// Simplify runs the retained NLSAT preprocessing schedule. Learned clauses are
// discarded first; factor splitting is delayed until a round strictly shrinks
// the original clause vector, exactly as in simplify::imp::operator().
func (s *Solver) Simplify() (summary SimplificationSummary, err error) {
	if err = s.open(); err != nil {
		return summary, err
	}
	if summary.LearnedRemoved, err = s.removeSimplifierLearnedClauses(); err != nil {
		return summary, err
	}
	prior := len(s.clauses.originals)
	for {
		summary.Rounds++
		round, cause := s.runSimplifierRound()
		if cause != nil {
			return summary, cause
		}
		summary.Subsumed += round.Subsumed
		summary.Unconstrained += round.Unconstrained
		summary.ContentRewritten += round.ContentRewritten
		summary.FourierMotzkin += round.FourierMotzkin
		if len(s.clauses.originals) >= prior {
			return summary, nil
		}
		split, cause := s.SimplifyUnitFactors()
		if cause != nil {
			return summary, cause
		}
		summary.FactorsSplit += split
		prior = len(s.clauses.originals)
	}
}
