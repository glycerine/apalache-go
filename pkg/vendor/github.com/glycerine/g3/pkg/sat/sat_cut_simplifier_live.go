package sat

type satCutSimplifierLive struct {
	solver *Solver
	aig    *satAIGCuts
	config satCutSimplifierConfig
	stats  satCutSimplifierStats
	closed bool
}

func newSATCutSimplifierLive(s *Solver) (*satCutSimplifierLive, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	a, e := newSATAIGCuts(s.budget)
	if e != nil {
		return nil, e
	}
	return &satCutSimplifierLive{solver: s, aig: a, config: defaultSATCutSimplifierConfig()}, nil
}
func (c *satCutSimplifierLive) Close() error {
	if c == nil || c.closed {
		return ErrSATOwner
	}
	c.closed = true
	return c.aig.Close()
}
