package sat

// Exchange preserves sat::par::exchange ordering: callers first receive the
// global suffix beginning at limit, then their input is deduplicated into the
// shared vector, and the returned limit observes the final vector size.
func (p *ParallelUnitExchange) Exchange(in []Literal, limit uint32) ([]Literal, uint32, error) {
	if p == nil {
		return nil, limit, ErrSATOwner
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.openLocked(); err != nil {
		return nil, limit, err
	}
	var out []Literal
	if limit < uint32(len(p.units)) {
		out = append(out, p.units[limit:]...)
	}
	for _, literal := range in {
		index := literal.index()
		if _, found := p.unitSet[index]; found {
			continue
		}
		p.unitSet[index] = struct{}{}
		p.units = append(p.units, literal)
	}
	return out, uint32(len(p.units)), nil
}
