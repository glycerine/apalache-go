package sat

func (p *ParallelUnitExchange) Len() (uint32, error) {
	if p == nil {
		return 0, ErrSATOwner
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.openLocked(); err != nil {
		return 0, err
	}
	return uint32(len(p.units)), nil
}

func (p *ParallelUnitExchange) Units() ([]Literal, error) {
	if p == nil {
		return nil, ErrSATOwner
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.openLocked(); err != nil {
		return nil, err
	}
	return append([]Literal(nil), p.units...), nil
}
