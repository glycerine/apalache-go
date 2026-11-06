package sat

func (l *satLUTFinder) setCombination(mask uint32) {
	bit := uint64(1) << mask
	if l.combination&bit == 0 {
		l.combination |= bit
		l.numCombinations++
	}
}
func (l *satLUTFinder) updateCombinations(mask uint32) bool {
	for k := uint32(0); k < uint32(1)<<uint32(len(l.missing)); k++ {
		m := mask
		for i, p := range l.missing {
			if k&(1<<uint32(i)) != 0 {
				m |= 1 << p
			}
		}
		l.setCombination(m)
	}
	return l.defined(uint32(len(l.vars)))
}
func (l *satLUTFinder) definedAt(i, size uint32) bool {
	c := l.combination | (l.combination >> (uint64(1) << i))
	m := l.masks[i]
	if size < 6 {
		m &= uint64(1)<<(uint64(1)<<size) - 1
	}
	return c&m == m
}
func (l *satLUTFinder) defined(size uint32) bool {
	if l.numCombinations < uint32(1)<<(size/2) {
		return false
	}
	for i := size; i > 0; i-- {
		if l.definedAt(i-1, size) {
			return true
		}
	}
	return false
}
