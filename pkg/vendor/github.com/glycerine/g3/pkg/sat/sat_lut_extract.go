package sat

func (l *satLUTFinder) extract(clause []Literal, pos map[BoolVar]uint32) bool {
	if !satLUTSubset(clause, pos) {
		return false
	}
	var mask uint32
	l.missing = l.missing[:0]
	present := make([]bool, len(l.vars))
	for _, x := range clause {
		p := pos[x.variable()]
		present[p] = true
		if x.sign() {
			mask |= 1 << p
		}
	}
	for i := range present {
		if !present[i] {
			l.missing = append(l.missing, uint32(i))
		}
	}
	return l.updateCombinations(mask)
}
func (l *satLUTFinder) convert() (uint64, []BoolVar, BoolVar) {
	size := uint32(len(l.vars))
	var pivot uint32
	for i := size; i > 0; i-- {
		if l.definedAt(i-1, size) {
			pivot = i - 1
			break
		}
	}
	v := l.vars[pivot]
	inputs := append([]BoolVar(nil), l.vars[:pivot]...)
	inputs = append(inputs, l.vars[pivot+1:]...)
	var table uint64
	off := uint32(0)
	m := l.masks[pivot]
	for j := uint32(0); j < 64; j++ {
		if m&(uint64(1)<<j) != 0 {
			if l.combination&(uint64(1)<<j) != 0 {
				table |= uint64(1) << off
			}
			off++
		}
	}
	return table, inputs, v
}
