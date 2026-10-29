package polynomial

// Source: src/math/polynomial/polynomial.cpp:467,474.

func (p *MonomialPositions) resetIndexAssert(id uint32) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	n, e := p.size()
	if e != nil || id < n {
		return e
	}
	return p.manager.sourceAssertionFailure(467, "id < m_m2pos.size()")
}
func (p *MonomialPositions) setEmptyAssert(id uint32) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	position, e := p.read(id)
	if e != nil || position == ^uint32(0) {
		return e
	}
	return p.manager.sourceAssertionFailure(474, "m_m2pos[id] == UINT_MAX")
}
