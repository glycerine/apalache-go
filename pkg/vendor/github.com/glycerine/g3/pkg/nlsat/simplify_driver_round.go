package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:38-53.

type simplifierRound struct{ Subsumed, Unconstrained, ContentRewritten, FourierMotzkin uint32 }

func (s *Solver) runSimplifierRound() (result simplifierRound, err error) {
	if result.Subsumed, err = s.SimplifySubsumedClauses(); err != nil {
		return result, err
	}
	if result.Unconstrained, _, err = s.SimplifyUnconstrainedEqualities(); err != nil {
		return result, err
	}
	if result.ContentRewritten, err = s.SimplifyLiteralContent(); err != nil {
		return result, err
	}
	if result.FourierMotzkin, _, err = s.SimplifyFourierMotzkin(); err != nil {
		return result, err
	}
	return result, nil
}
